import os
import threading
import time

from redis import Redis

from .client import SchedulerClient
from .executor import GPUBenchmarkSimulator
from .models import WorkerConfig


def load_config() -> WorkerConfig:
    return WorkerConfig(
        worker_id=os.getenv("WORKER_ID", "gpu_node_01"),
        gpu=os.getenv("GPU_TYPE", "A100"),
        memory=os.getenv("GPU_MEMORY", "80GB"),
        status="available",
        max_concurrency=int(os.getenv("MAX_CONCURRENCY", "1")),
    )


def heartbeat_loop(client: SchedulerClient, config: WorkerConfig, state: dict):
    """
    Workflow Automation: Heartbeat Loop
    Runs as a daemon thread. Periodically sends worker status to the Kubernetes Operator.
    This guarantees that if the worker node crashes, the Operator will eventually
    time it out and automatically reschedule its workloads (Self-Healing).
    """
    while True:
        payload = {
            "worker_id": config.worker_id,
            "gpu": config.gpu,
            "memory": config.memory,
            "status": "available" if state["running_jobs"] < config.max_concurrency else "busy",
            "running_jobs": state["running_jobs"],
            "max_concurrency": config.max_concurrency,
        }
        try:
            client.send_heartbeat(payload)
        except Exception as ex:
            print(f"heartbeat error: {ex}")
        time.sleep(5)


def main():
    scheduler_url = os.getenv("SCHEDULER_URL", "http://localhost:8080")
    redis_addr = os.getenv("REDIS_ADDR", "localhost")
    redis_port = int(os.getenv("REDIS_PORT", "6379"))

    config = load_config()
    state = {"running_jobs": 0}

    scheduler = SchedulerClient(scheduler_url)
    redis_client = Redis(host=redis_addr, port=redis_port, decode_responses=True)
    executor = GPUBenchmarkSimulator()

    scheduler.register_worker(
        {
            "worker_id": config.worker_id,
            "gpu": config.gpu,
            "memory": config.memory,
            "status": config.status,
            "max_concurrency": config.max_concurrency,
        }
    )

    threading.Thread(target=heartbeat_loop, args=(scheduler, config, state), daemon=True).start()

    queue_key = f"worker:{config.worker_id}:jobs"
    print(f"worker {config.worker_id} listening on {queue_key}")

    while True:
        item = redis_client.blpop(queue_key, timeout=3)
        if not item:
            continue

        _, job_id = item
        try:
            state["running_jobs"] += 1
            job = scheduler.get_job(job_id)
            result = executor.execute(job.get("workload", "unknown"))
            scheduler.report_result(job_id=job_id, status=result["status"], error=result.get("error", ""))
        except Exception as ex:
            scheduler.report_result(job_id=job_id, status="failed", error=str(ex))
        finally:
            state["running_jobs"] = max(0, state["running_jobs"] - 1)


if __name__ == "__main__":
    main()
