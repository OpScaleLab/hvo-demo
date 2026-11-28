# hvo-demo

A small Go web app that shows what [HVO](https://github.com/OpScaleLab/hvo)
gives every app: build from source (no Dockerfile), dev → prod with one
approval, PR previews, Postgres, Redis and a bucket, traces, logs, browser
vitals and session replay. Everything it needs from the platform is in
[`hvo.yaml`](hvo.yaml).

Live: <https://hvo-demo.nalyx.dev> (signed-in HVO users can browse it read
only in the portal).

## Use it as a starter

1. **Use this template → Create a new repository** (public, or private with
   the HVO GitHub App installed).
2. In the HVO portal: **Add app** → paste your repository URL → **Register**.
3. Watch the steps on the app page. The first build takes a few minutes;
   your copy goes live on `dev-hvo-demo-<company>.nalyx.dev`, then
   `hvo-demo-<company>.nalyx.dev` after you approve prod.

No edits are needed: the host `hvo-demo.nalyx.dev` belongs to OpScale's
copy, so HVO assigns yours, and the greeting secrets are `optionalSecrets`
(the sealed ones here are OpScale's and are skipped in your copy). To use
your own name, set `expose.host` to a free `<name>.nalyx.dev` or a host
under your verified domain.

## Secrets

`GREETING` comes from Secret `hvo-demo-config`, `SEALED_GREETING` from a
[Keyfold](https://github.com/OpScaleLab/hvo) sealed secret committed under
`.hvo/secrets/<env>/`. Both are optional; reseal for your own namespace
(`team-<company>-<team>-<env>`) to show them in your copy.
