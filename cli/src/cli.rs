use clap::{Parser, Subcommand};
use std::path::PathBuf;

#[derive(Debug, Parser)]
pub struct CC {
    #[clap(subcommand)]
    pub command: Option<CCCommand>,
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
        path: PathBuf,
    },
}

#[derive(Debug, Subcommand)]
pub enum ControllerCommand {
    Install,
    Uninstall,
}

#[derive(Debug, Subcommand)]
pub enum TaskCommand {
    Create,
    Delete,
}
