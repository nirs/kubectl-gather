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
