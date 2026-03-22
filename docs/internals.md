# kubectl-gather internals

## Pipeline

kubectl-gather fetch and process data in a 3 steps pipeline:

```
prepare --> gather --> inpsect
```

### Prepare step

This step runs in the goroutine calling Gather(). This step includes:

1. Looking up available namespaces - if no namespaces is given, we have list
   with one iteme, the special empty namespace, gathering resources from all
   namespaces.
1. Looking up API resources.
1. Queuing gatherResources(resource, namespace) for every resource and namespace in the gatherWorkqueu.
1. Close the gatherWorkqueue

This step ends when all work was queued in the resources work queue. Since the queue is unbuffered, this ends when the last gatherResources() call was queued.

### Gather step

This steps run in the gatherWorkqueue goroutines.

The workers pick up work functions from the queue and run them.

Running gather resources steps:

1. List all resources with specified type and namespaces
1. Dump every resource to the output directory
1. If an addon is registered for the resource, call addon.Inspect(). Inspecting
   a resource may fetch new resources from the cluster, or queue more work in
   the inspectWorkqueue.
1. When all workers are done, close the inspectWorkqueue

Workers cannot queue more work in the gatherWorkqueue. All work must be queued in prepare step.

The workers exit when there is no more work to do.

### Inspect step

This step runs in the inspectWorkqueue.

The workers running in this step pick up work functions from the queue and run them.

Work done in the inspect queue depends on the addon. Examples are:

- Copy logs from containers
- Running commands in agenet pod
- Copy logs from nodes

Workers cannot queue more work in the inspectWorkqueue. All work must be queued in gather step.

The workers exit when there is no more work to do.

## Cancellation

`New()` accepts a `context.Context` and passes it to the work queues
and all components. When the context is cancelled (e.g. via
SIGINT/SIGTERM or `--timeout`), gathering stops at three layers:

### Queue

`Queue()` uses `select` on `ctx.Done()` to silently drop new work
when the context is cancelled. Producers never block on a cancelled
context.

### Workers

Workers process any in-flight work normally. The work functions
themselves (API calls, exec.CommandContext, streaming) return quickly
when the context is cancelled. Since the channel is unbuffered, at
most one item per worker can be in flight at cancellation time.

### Loops

`gatherResources` checks `ctx.Err()` at the top of each iteration in
both the pagination loop and the per-item loop, breaking out of local
processing that does not touch the network.

### Cleanup

Functions that modify cluster state (e.g. creating agent pods) must
clean up even after cancellation. `agent.Delete()` uses
`context.Background()` instead of the gather context so the API call
succeeds. Any future cluster modifications must follow the same
pattern: `defer` cleanup with a fresh context.

### Partial results

When cancelled mid-gather, the output directory contains partial data.
A warning is logged (e.g. "Gather cancelled, results are partial") and
the context error is returned to the caller.
