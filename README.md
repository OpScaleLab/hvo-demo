# hvo-demo

Minimal workload for exercising an [HVO](https://github.com/OpScaleLab/hvo)
ApplicationWorkflow end to end on think-lab:

1. An `ApplicationWorkflow` starts the `hvo-demo-build` Tekton Pipeline,
   which clones this repo and builds/pushes the image to the in-cluster zot
   registry (`zot.registry.svc.cluster.local:5000/hvo-demo/web`).
2. HVO rewrites `deploy/deployment.yaml` to the pushed digest and commits it
   to `main`.
3. HVO asks Argo CD (`argocd/hvo-demo`) to refresh, and Argo rolls it out.

The pipeline, Argo app and HVO `Application` live in think-lab-gitops
(`addons/hvo-demo`, `gitops/apps/hvo-demo.yaml`).

`deploy/deployment.yaml` is written by HVO; don't hand-edit the image.
