use kube::CustomResource;
use schemars::JsonSchema;
use serde::{Deserialize, Serialize};

#[derive(Debug, Serialize, Deserialize, JsonSchema, Clone, Default)]
#[serde(rename_all = "camelCase")]
pub struct JobSpec {
    pub image: String,
    pub command: Vec<String>,
    #[serde(default)]
    pub args: Vec<String>,
    pub backoff_limit: u32,
    pub image_pull_policy: Option<String>,
}

#[derive(CustomResource, Debug, Serialize, Deserialize, Clone, JsonSchema)]
#[kube(
    group = "classroom.flyinpancake.com",
    version = "v1",
    kind = "ClusterClassroom",
    status = "ClusterClassroomStatus"
)]
#[serde(rename_all = "camelCase")]
pub struct ClusterClassroomSpec {
    pub namespace_prefix: String,
    pub constructor: JobSpec,
    pub evaluator: JobSpec,
    pub student_id: String,
}

#[derive(Debug, Serialize, Deserialize, JsonSchema, Clone, Default)]
#[serde(rename_all = "camelCase")]
pub struct ClusterClassroomStatus {
    pub namespace: String,
    pub namespace_phase: String,
    pub constructor_job_phase: String,
    pub evaluator_job_phase: String,
}
