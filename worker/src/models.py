from dataclasses import dataclass


@dataclass
class WorkerConfig:
    worker_id: str
    gpu: str
    memory: str
    status: str = "available"
    max_concurrency: int = 1


@dataclass
class Job:
    id: str
    workload: str
    gpu_requirement: str
    priority: int
