use std::{collections::BTreeMap, path::PathBuf};

use clap::Parser;
use cli::{ControllerCommand, TaskCommand};
use config::{ClusterClassroom, ClusterClassroomSpec, ClusterClassroomStatus};
use k8s_openapi::api::{
    apps::v1::Deployment,
    batch::v1::Job,
    core::v1::{Namespace, Pod},
};
use kube::{
    api::{DeleteParams, ListParams, ObjectMeta, Patch, PatchParams},
    Api,
};
use serde_json::json;
use tracing::{error, info, warn};

mod cli;
mod config;

const CONTROLLER_MANIFEST: &str = "https://raw.githubusercontent.com/FlyinPancake/ClusterClassroom/refs/heads/main/controller/dist/install.yaml";

#[tokio::main]
async fn main() -> anyhow::Result<()> {
    tracing_subscriber::fmt()
        .pretty()
        .with_target(false)
        .with_thread_ids(false)
        .with_file(false)
        .with_line_number(false)
        .with_level(true)
        .with_ansi(true)
        .with_timer(())
        // .with_timer(None)
        .compact()
        .init();
    let args = cli::CC::parse();

    match args.command {
        cli::CCCommand::Controller(ControllerCommand::Install) => {
            // Run kubectl apply -f controller.yaml
            install_controller().await
        }
        cli::CCCommand::Controller(ControllerCommand::Uninstall) => {
            // Run kubectl delete -f controller.yaml
            uninstall_controller()
        }
        cli::CCCommand::Task {
            command: TaskCommand::Create { path },
        } => create_task_cr(path).await,

        cli::CCCommand::Task {
            command: TaskCommand::Delete { path },
        } => delete_task_cr(path).await,
        cli::CCCommand::Task {
            command: TaskCommand::Test { path, keep },
        } => test_task_cr(path, keep).await,
        #[cfg(feature = "schema")]
        cli::CCCommand::WriteSchemas => {
            let schema_out_dir = PathBuf::from("../docs/schemas");
            let task_schema = schemars::schema_for!(ClusterClassroomSpec);
            // write the schema to a file
            std::fs::create_dir_all(&schema_out_dir)?;
            let schema_file = schema_out_dir.join("task.json");
            let schema_file = std::fs::File::create(schema_file)?;
            serde_json::to_writer_pretty(schema_file, &task_schema)?;
            Ok(())
        }
    }
}

async fn test_task_cr(path: PathBuf, keep: bool) -> anyhow::Result<()> {
    let cc: ClusterClassroomSpec = serde_yml::from_reader(std::fs::File::open(path)?)?;

    let kube_client = kube::Client::try_default()
        .await
        .map_err(|e| anyhow::anyhow!(e))?;

    let cluster_classrooms: Api<ClusterClassroom> = Api::all(kube_client.clone());
    let cc_name = format!("{}-{}", cc.namespace_prefix, cc.student_id).to_lowercase();

    let jobs_api = Api::<Job>::namespaced(kube_client.clone(), &cc_name);
    let cc_evaluator_job_name = format!("{}-evaluator-job", cc_name);
    let evaluator_job = jobs_api.get(&cc_evaluator_job_name).await?;
    // get the evaluator job's suspended status
    if let Some(spec) = evaluator_job.spec {
        if !spec.suspend.unwrap_or(false) {
            info!("Recreating evaluator job...");
            let delete_params = DeleteParams {
                propagation_policy: Some(kube::api::PropagationPolicy::Foreground),
                ..Default::default()
            };
            let _ = jobs_api
                .delete(&cc_evaluator_job_name, &delete_params)
                .await?;
            while let Some(ClusterClassroomStatus {
                evaluator_job_phase: eval_phase,
                ..
            }) = cluster_classrooms
                .get_status(&cc_name)
                .await?
                .status
                .as_ref()
            {
                if eval_phase == "Pending" {
                    break;
                }
                tokio::time::sleep(std::time::Duration::from_secs(1)).await;
            }
        }
        info!("Starting evaluation!");
        let patch = json!({
            "spec": {
                "suspend": false
            }
        });
        // apply the patch to unsuspend the job
        let patch_params = PatchParams::default();
        let _ = jobs_api
            .patch(&cc_evaluator_job_name, &patch_params, &Patch::Merge(patch))
            .await?;

        while let Some(ClusterClassroomStatus {
            evaluator_job_phase: eval_phase,
            ..
        }) = cluster_classrooms
            .get_status(&cc_name)
            .await?
            .status
            .as_ref()
        {
            if eval_phase == "Completed" {
                break;
            }
            tokio::time::sleep(std::time::Duration::from_secs(1)).await;
        }

        info!("Evaluator job completed!");

        // let evaluator_job = jobs_api.get(&cc_evaluator_job_name).await?;

        // get the evaluator job's logs
        let pod_api = Api::<Pod>::namespaced(kube_client.clone(), &cc_name);
        let pod_list = pod_api
            .list(&ListParams::default().labels(&format!("job-name={}", &cc_evaluator_job_name)))
            .await?;

        let evaluator_pod = pod_list.items.first().unwrap();
        let logs = pod_api
            .logs(
                evaluator_pod
                    .metadata
                    .name
                    .as_ref()
                    .ok_or_else(|| anyhow::anyhow!("Pod not found"))?,
                &Default::default(),
            )
            .await?;
        info!("Evaluator logs: \n{}", logs);
        // delete the evaluator job

        if !keep {
            let delete_params = DeleteParams {
                propagation_policy: Some(kube::api::PropagationPolicy::Foreground),
                ..Default::default()
            };
            let _ = jobs_api
                .delete(&cc_evaluator_job_name, &delete_params)
                .await?;
        }
    }
    // let cc = cluster_classrooms.get(&cc_name).await?;
    // if let Some(status) = cc.status {
    //     if "Pending" == status.evaluator_job_phase {
    //     } else {
    //         return Err(anyhow::anyhow!("Evaluation job not found!"));
    //     }
    // } else {
    //     return Err(anyhow::anyhow!("Namespace `{}` does not exist!", cc_name));
    // }

    Ok(())
}

async fn install_controller() -> anyhow::Result<()> {
    let output = std::process::Command::new("kubectl")
        .args(["apply", "-f", CONTROLLER_MANIFEST])
        .output()
        .expect("Failed to execute kubectl apply -f controller.yaml");
    if !output.status.success() {
        let error_str = String::from_utf8(output.stderr)?;
        error!("{}", error_str);
        return Err(anyhow::anyhow!("Failed to install controller"));
    }
    info!("Controller installed!");
    info!("Waiting for the controller to be ready...");

    let kube_client = kube::Client::try_default()
        .await
        .map_err(|e| anyhow::anyhow!(e))?;

    let apps = Api::<Deployment>::namespaced(kube_client.clone(), "controller-system");
    let mut ready = false;
    while !ready {
        let deployment = apps.get("controller-controller-manager").await?;
        if let Some(status) = deployment.status {
            if let Some(available) = status.available_replicas {
                if available > 0 {
                    ready = true;
                }
            }
        }
        tokio::time::sleep(std::time::Duration::from_secs(1)).await;
    }
    info!("Controller is ready!");

    Ok(())
}

fn uninstall_controller() -> anyhow::Result<()> {
    let output = std::process::Command::new("kubectl")
        .args(["delete", "-f", CONTROLLER_MANIFEST])
        .output()
        .expect("Failed to execute kubectl delete -f controller.yaml");
    if !output.status.success() {
        let error_str = String::from_utf8(output.stderr)?;
        error!("{}", error_str);
        return Err(anyhow::anyhow!("Failed to uninstall controller"));
    }
    info!("Controller uninstalled!");
    Ok(())
}

async fn delete_task_cr(path: PathBuf) -> anyhow::Result<()> {
    let cc: ClusterClassroomSpec = serde_yml::from_reader(std::fs::File::open(path)?)?;

    let kube_client = kube::Client::try_default()
        .await
        .map_err(|e| anyhow::anyhow!(e))?;

    let cluster_classrooms: Api<ClusterClassroom> = Api::all(kube_client.clone());
    let cc_name = format!("{}-{}", cc.namespace_prefix, cc.student_id).to_lowercase();
    let cc = ClusterClassroom {
        metadata: ObjectMeta {
            name: Some(cc_name.clone()),
            ..Default::default()
        },
        spec: cc,
        status: None,
    };
    let _ = cluster_classrooms
        .delete(&cc.metadata.name.unwrap(), &Default::default())
        .await?;

    info!("Task descriptor deleted!");
    warn!("Waiting for the namespace to be deleted...");
    let namespaces = Api::<Namespace>::all(kube_client.clone());
    while namespaces.get(&cc_name).await.is_ok() {
        tokio::time::sleep(std::time::Duration::from_secs(1)).await;
    }
    info!("Namespace `{}` deleted!", cc_name);

    Ok(())
}

async fn create_task_cr(path: PathBuf) -> anyhow::Result<()> {
    let cc: ClusterClassroomSpec = serde_yml::from_reader(std::fs::File::open(path)?)?;

    let kube_client = kube::Client::try_default()
        .await
        .map_err(|e| anyhow::anyhow!(e))?;

    let cluster_classrooms: Api<ClusterClassroom> = Api::all(kube_client.clone());
    let cc_name = format!("{}-{}", cc.namespace_prefix, cc.student_id.to_lowercase()).to_string();
    let cc = ClusterClassroom {
        metadata: ObjectMeta {
            name: Some(cc_name.clone()),
            labels: Some(BTreeMap::from_iter([
                (
                    "app.kubernetes.io/name".to_string(),
                    "clusterclassroom".to_string(),
                ),
                (
                    "app.kubernetes.io/managed-by".to_string(),
                    "kustomize".to_string(),
                ),
            ])),
            ..Default::default()
        },
        spec: cc,
        status: None,
    };
    let mut cc = cluster_classrooms.create(&Default::default(), &cc).await?;

    while cc.status.is_none() {
        tokio::time::sleep(std::time::Duration::from_secs(1)).await;
        cc = cluster_classrooms.get(&cc_name).await?;
    }
    info!("Task descriptor created!");

    while let Some(status) = cc.status.as_ref() {
        if status.namespace_phase == "Created" {
            info!("Namespace `{}` created!", status.namespace);
            break;
        }
        tokio::time::sleep(std::time::Duration::from_secs(1)).await;
        cc = cluster_classrooms.get(&cc_name).await?;
    }
    info!("Setting up environment...");
    while let Some(status) = cc.status.as_ref() {
        if status.constructor_job_phase == "Completed" {
            info!("Constructor job succeeded!");
            break;
        }
        tokio::time::sleep(std::time::Duration::from_secs(1)).await;
        cc = cluster_classrooms.get(&cc_name).await?;
    }
    info!("Environment setup complete!");
    info!("Remember to change the namespace to `{}`", cc_name);

    Ok(())
}
