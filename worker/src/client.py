import time
import requests

class SchedulerClient:
    """
    SchedulerClient: Communicates with the Kubernetes Operator / Control Plane.
    
    Fault Tolerance & Automation:
    - Incorporates Exponential Backoff Retry mechanism for transient network failures.
    - Handles heartbeat state reporting to maintain GPUPool available capacity.
    """
    def __init__(self, base_url: str):
        self.base_url = base_url.rstrip("/")

    def _request_with_retry(self, method: str, endpoint: str, max_retries: int = 3, **kwargs):
        """
        Executes HTTP requests with Exponential Backoff Retry.
        Guarantees the worker does not crash on temporary control plane unavailability.
        """
        url = f"{self.base_url}{endpoint}"
        for attempt in range(max_retries):
            try:
                response = requests.request(method, url, timeout=5, **kwargs)
                response.raise_for_status()
                return response
            except requests.RequestException as e:
                if attempt == max_retries - 1:
                    raise e
                time.sleep(2 ** attempt)

    def register_worker(self, payload: dict) -> None:
        self._request_with_retry("POST", "/workers/register", json=payload)

    def send_heartbeat(self, payload: dict) -> None:
        """
        Heartbeat Mechanism: 
        Continuously syncs worker state (running jobs, status) to the Operator.
        If heartbeat is lost, the Operator will reclaim the GPU capacity.
        """
        self._request_with_retry("POST", "/workers/heartbeat", json=payload)

    def get_job(self, job_id: str) -> dict:
        response = self._request_with_retry("GET", f"/jobs/{job_id}")
        return response.json()

    def report_result(self, job_id: str, status: str, error: str = "") -> None:
        self._request_with_retry(
            "POST",
            f"/jobs/{job_id}/result",
            json={"status": status, "error": error}
        )
