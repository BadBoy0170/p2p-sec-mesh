"""
gRPC server for the AI Analyzer Sidecar.

Listens on port 50052 (configurable via SIDECAR_PORT env var).
Connects to Ollama via the LLM client for threat analysis.
"""
import sys
import os
import time
import json
from concurrent import futures

import grpc

# ── Stub path setup ───────────────────────────────────────────────────────────
_STUBS_DIR = os.path.join(os.path.dirname(__file__), "stubs")
if _STUBS_DIR not in sys.path:
    sys.path.insert(0, _STUBS_DIR)

try:
    import sidecar_pb2
    import sidecar_pb2_grpc
except ImportError:
    print("ERROR: gRPC stubs not found in ./stubs/", file=sys.stderr)
    print(
        "Run: python -m grpc_tools.protoc -I../protos "
        "--python_out=./stubs --grpc_python_out=./stubs ../protos/sidecar.proto",
        file=sys.stderr,
    )
    sys.exit(1)

from llm_client import analyze_log_with_llm

_SIDECAR_PORT = os.getenv("SIDECAR_PORT", "50052")
_ONE_DAY = 60 * 60 * 24


class AISidecarService(sidecar_pb2_grpc.AISidecarServicer):
    """Implements the AISidecar gRPC service."""

    def AnalyzeTelemetry(self, request, context):
        node_id = request.node_id
        print(f"[SIDECAR] Received telemetry from node: {node_id}")

        # Build a human-readable summary for the LLM
        events = []
        for event in request.events:
            events.append({
                "type":      sidecar_pb2.CriticalEvent.EventType.Name(event.type),
                "source_ip": event.source_ip,
                "timestamp": event.timestamp,
            })

        telemetry = {
            "node_id":        node_id,
            "cpu_usage_pct":  f"{request.cpu_usage:.2f}",
            "mem_usage_pct":  f"{request.mem_usage:.2f}",
            "critical_events": events,
        }
        log_content = json.dumps(telemetry, indent=2)

        threat_score = analyze_log_with_llm(log_content)

        if threat_score == -1:
            context.set_code(grpc.StatusCode.INTERNAL)
            context.set_details("LLM analysis failed.")
            return sidecar_pb2.ThreatAnalysis()

        print(f"[SIDECAR] Threat score for {node_id}: {threat_score}/10")
        return sidecar_pb2.ThreatAnalysis(
            node_id=node_id,
            threat_score=float(threat_score),
        )


def serve():
    server = grpc.server(futures.ThreadPoolExecutor(max_workers=10))
    sidecar_pb2_grpc.add_AISidecarServicer_to_server(AISidecarService(), server)

    listen_addr = f"[::]:{_SIDECAR_PORT}"
    server.add_insecure_port(listen_addr)

    print(f"[SIDECAR] AI Sidecar gRPC server listening on port {_SIDECAR_PORT}")
    server.start()

    try:
        while True:
            time.sleep(_ONE_DAY)
    except KeyboardInterrupt:
        print("[SIDECAR] Stopping...")
        server.stop(grace=5)


if __name__ == "__main__":
    if "--help" in sys.argv:
        print("Usage: python app.py")
        print("Env vars: SIDECAR_PORT, OLLAMA_URL, OLLAMA_MODEL")
        sys.exit(0)
    serve()
