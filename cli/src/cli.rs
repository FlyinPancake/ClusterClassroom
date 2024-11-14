use clap::{Parser, Subcommand};
use std::path::PathBuf;

#[derive(Debug, Parser)]
pub struct CC {
    #[clap(subcommand)]
    pub command: CCCommand,
    #[clap(short, long)]
    /// The path to the config file
    ///
    /// If not provided, the default path is `~/.config/clusterclassroom/cccli.yaml` or `~/.cccli.yaml`.
    ///
    /// If both are present, the `~/.config/clusterclassroom/cccli.yaml` will be used.
    ///
    /// If the file does not exist, it will be created.
    pub config: Option<String>,
}

#[derive(Debug, Subcommand)]
pub enum CCCommand {
    #[cfg(feature = "schema")]
    WriteSchemas,
    /// Install the ClusterClassroom Controller into the cluster
    #[clap(subcommand)]
    Controller(ControllerCommand),
    /// Set up a task for a student
    Task {
        #[clap(subcommand)]
        command: TaskCommand,
    },
}

#[derive(Debug, Subcommand)]
pub enum ControllerCommand {
    /// Install the ClusterClassroom Controller into the cluster
    ///
    /// This will run `kubectl apply -f controller.yaml`
    /// with controller.yaml replaced with the actual path to the controller manifest.
    Install,
    /// Uninstall the ClusterClassroom Controller from the cluster
    ///
    /// This will uninstall ClusterClassroom using `kubectl delete -f controller.yaml`
    /// with the default kubernetes context.
    Uninstall,
}

#[derive(Debug, Subcommand)]
pub enum TaskCommand {
    /// Create a task for a student
    Create {
        /// The path to the task configuration file
        path: PathBuf,
    },
    /// Delete a task for a student
    Delete {
        /// The path to the task configuration file
        path: PathBuf,
    },
    /// Run tests for a task
    Test {
        /// The path to the task configuration file
        path: PathBuf,
        #[clap(short, long, default_value = "false")]
        /// Keep the evaluation pod after the test is done
        keep: bool,
    },
}
