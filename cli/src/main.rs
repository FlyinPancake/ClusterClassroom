use anyhow;
use clap::Parser;
use cli::{ControllerCommand, TaskCommand};
use config::{ClusterClassroom, ClusterClassroomSpec};
use kube::{api::ObjectMeta, Api};
use tracing::{info, warn};

mod cli;
mod config;

const CONTROLLER_MANIFEST: &str = "https://raw.githubusercontent.com/FlyinPancake/ClusterClassroom/refs/heads/main/controller/dist/install.yaml";

#[tokio::main]
async fn main() -> anyhow::Result<()> {
    tracing_subscriber::fmt().pretty().init();
    let args = cli::CC::parse();

    if let None = args.command {
        warn!("No command provided");
        return Ok(());
    };

    match args.command.unwrap() {
        cli::CCCommand::Controller(ControllerCommand::Install) => {
            // Run kubectl apply -f controller.yaml
            let _output = std::process::Command::new("kubectl")
                .args(&["apply", "-f", CONTROLLER_MANIFEST])
                .output()
                .expect("Failed to execute kubectl apply -f controller.yaml");
            info!("Controller installed!");
            return Ok(());
        }
        cli::CCCommand::Controller(ControllerCommand::Uninstall) => {
            // Run kubectl delete -f controller.yaml
            let _output = std::process::Command::new("kubectl")
                .args(&["delete", "-f", CONTROLLER_MANIFEST])
                .output()
                .expect("Failed to execute kubectl delete -f controller.yaml");
            info!("Controller uninstalled!");
            return Ok(());
        }
        cli::CCCommand::Task {
            command: TaskCommand::Create,
            path,
        } => {
            let cc: ClusterClassroomSpec = serde_yml::from_reader(std::fs::File::open(path)?)?;

            let kube_client = kube::Client::try_default()
                .await
                .map_err(|e| anyhow::anyhow!(e))?;

            let cluster_classrooms: Api<ClusterClassroom> = Api::all(kube_client.clone());
            let cc = ClusterClassroom {
                metadata: ObjectMeta {
                    name: Some(format!("{}-{}", cc.namespace_prefix, cc.student_id).to_string()),
                    ..Default::default()
                },
                spec: cc,
            };
            cluster_classrooms.create(&Default::default(), &cc).await?;

            return Ok(());
        }

        cli::CCCommand::Task {
            command: TaskCommand::Delete,
            path,
        } => {
            let cc: ClusterClassroomSpec = serde_yml::from_reader(std::fs::File::open(path)?)?;

            let kube_client = kube::Client::try_default()
                .await
                .map_err(|e| anyhow::anyhow!(e))?;

            let cluster_classrooms: Api<ClusterClassroom> = Api::all(kube_client.clone());
            let cc = ClusterClassroom {
                metadata: ObjectMeta {
                    name: Some(format!("{}-{}", cc.namespace_prefix, cc.student_id).to_string()),
                    ..Default::default()
                },
                spec: cc,
            };
            cluster_classrooms
                .delete(&cc.metadata.name.unwrap(), &Default::default())
                .await?;

            return Ok(());
        }
    }

    // let crds: Api<CustomResourceDefinition> = Api::all(kube_client.clone());
    // for item in crds.list(&Default::default()).await? {
    //     println!("{:?}", item.metadata.name);
    // }

    // let cluster_classrooms: Api<ClusterClassroom> = Api::all(kube_client);
    // // println!("{}", serde_yml::to_string(&ClusterClassroom::crd())?);
    // let ccs = cluster_classrooms.list(&Default::default()).await?;
    // for cc in ccs {
    //     info!("ClusterClassroom: \n{}", serde_yml::to_string(&cc.spec)?);
    // }

    // Ok(())
}
