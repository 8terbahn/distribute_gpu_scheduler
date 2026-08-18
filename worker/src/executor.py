import random
import time
from abc import ABC, abstractmethod


class JobExecutor(ABC):
    @abstractmethod
    def execute(self, workload: str) -> dict:
        raise NotImplementedError


class GPUBenchmarkSimulator(JobExecutor):
    def execute(self, workload: str) -> dict:
        if workload == "llm_inference":
            compute_time = random.uniform(0.8, 2.2)
        else:
            compute_time = random.uniform(0.3, 1.2)

        time.sleep(compute_time)
        failure = random.random() < 0.15
        if failure:
            return {
                "status": "failed",
                "error": "simulated transient GPU execution failure",
            }

        tokens_per_second = round(random.uniform(45.0, 160.0), 2)
        return {
            "status": "completed",
            "error": "",
            "metrics": {
                "duration_seconds": round(compute_time, 2),
                "throughput_tokens_per_second": tokens_per_second,
            },
        }
