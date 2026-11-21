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

## Now: just source code and hvo.yaml

There is no Dockerfile and no Kubernetes manifest here any more. HVO builds the
code with Cloud Native Buildpacks and deploys it from `hvo.yaml` to dev on
every push (https://dev-hvo-demo.think-lab.opscale.ir), then to prod after one
approval (https://hvo-demo.think-lab.opscale.ir). Pull requests get a preview
in dev.
