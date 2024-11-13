use std::{collections::BTreeMap, path::PathBuf};

use clap::Parser;
use cli::{ControllerCommand, TaskCommand};
use config::{ClusterClassroom, ClusterClassroomSpec};
use kube::{api::ObjectMeta, Api};
use tracing::info;

mod cli;
mod config;

const CONTROLLER_MANIFEST: &str = "https://raw.githubusercontent.com/FlyinPancake/ClusterClassroom/refs/heads/main/controller/dist/install.yaml";

#[tokio::main]
async fn main() -> anyhow::Result<()> {
    tracing_subscriber::fmt().pretty().init();
    let args = cli::CC::parse();

    match args.command {
        cli::CCCommand::Controller(ControllerCommand::Install) => {
            // Run kubectl apply -f controller.yaml
            let _output = std::process::Command::new("kubectl")
                .args(["apply", "-f", CONTROLLER_MANIFEST])
                .output()
                .expect("Failed to execute kubectl apply -f controller.yaml");
            info!("Controller installed!");
            return Ok(());
        }
        cli::CCCommand::Controller(ControllerCommand::Uninstall) => {
            // Run kubectl delete -f controller.yaml
            let _output = std::process::Command::new("kubectl")
                .args(["delete", "-f", CONTROLLER_MANIFEST])
                .output()
                .expect("Failed to execute kubectl delete -f controller.yaml");
            info!("Controller uninstalled!");
            return Ok(());
        }
        cli::CCCommand::Task {
            command: TaskCommand::Create { path },
        } => create_task_cr(path).await,

        cli::CCCommand::Task {
            command: TaskCommand::Delete { path },
        } => delete_task_cr(path).await,
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

async fn delete_task_cr(path: PathBuf) -> anyhow::Result<()> {
    let cc: ClusterClassroomSpec = serde_yml::from_reader(std::fs::File::open(path)?)?;

    let kube_client = kube::Client::try_default()
        .await
        .map_err(|e| anyhow::anyhow!(e))?;

    let cluster_classrooms: Api<ClusterClassroom> = Api::all(kube_client.clone());
    let cc = ClusterClassroom {
        metadata: ObjectMeta {
            name: Some(
                format!("{}-{}", cc.namespace_prefix, cc.student_id)
                    .to_string()
                    .to_lowercase(),
            ),
            ..Default::default()
        },
        spec: cc,
    };
    cluster_classrooms
        .delete(&cc.metadata.name.unwrap(), &Default::default())
        .await?;

    Ok(())
}

async fn create_task_cr(path: PathBuf) -> anyhow::Result<()> {
    let cc: ClusterClassroomSpec = serde_yml::from_reader(std::fs::File::open(path)?)?;

    let kube_client = kube::Client::try_default()
        .await
        .map_err(|e| anyhow::anyhow!(e))?;

    let cluster_classrooms: Api<ClusterClassroom> = Api::all(kube_client.clone());
    let cc = ClusterClassroom {
        metadata: ObjectMeta {
            name: Some(
                format!("{}-{}", cc.namespace_prefix, cc.student_id.to_lowercase()).to_string(),
            ),
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
    };
    cluster_classrooms.create(&Default::default(), &cc).await?;

    Ok(())
}
