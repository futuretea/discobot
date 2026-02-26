#!/bin/sh
# Entrypoint for the BuildKit container.
# Starts containerd first, then runs buildkitd with the containerd worker
# instead of the default OCI/runc worker.
set -e

# Start containerd in the background
containerd &

# Wait for the containerd socket to be ready
timeout=30
while [ ! -S /run/containerd/containerd.sock ] && [ "$timeout" -gt 0 ]; do
    sleep 0.1
    timeout=$((timeout - 1))
done

if [ ! -S /run/containerd/containerd.sock ]; then
    echo "ERROR: containerd socket not available after 3 seconds" >&2
    exit 1
fi

# Run buildkitd with containerd worker, disabling the default OCI/runc worker.
exec buildkitd \
    --oci-worker=false \
    --containerd-worker=true \
    --containerd-worker-addr=/run/containerd/containerd.sock \
    "$@"
