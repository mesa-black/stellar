# Running stellar.mesa.black

How the site is kept up, what tells us when it is not, and what to do then.
This is about our own deployment; `README.md` says how to run Stellar
anywhere.

## What we aim for

| | Aim | How it is known |
| --- | --- | --- |
| The site answers | 99.5 % of the half-hourly probes in a month: about three and a half hours down at most | The `Probe` workflow |
| A page for a host already read | under 300 ms for 95 in 100 | `make image`, then the load test below |
| Memory | under 48 MiB of the 64 the pod may use | The load test, and `kubectl top` |
| A push to `main` | served within 15 minutes, or the pipeline is red | The `Served, and answering` job |

These are aims, written down so that a change can be judged against them.
Nobody is paged: a failure is a mail.

## How it runs

Two pods of the same image, behind one Service, on one node. They share
nothing: each keeps its own five-minute memory of the keys it read. One that
crashes or is killed leaves the other answering while it restarts; the node
itself going down takes both. Each may use 64 MiB, and `GOMEMLIMIT=48MiB`
tells the Go runtime where to stop: it does not read the container's limit,
and would otherwise let the heap grow until the kernel killed the pod.

## What watches

- **`Probe`**, every half hour, from GitHub's machines (`.github/workflows/probe.yml`,
  `tools/probe`): the site answers, it can read a host, the key it serves is
  confirmed and another is refused, no sentence is broken, the certificate
  has more than fourteen days left. A failure is a mail from GitHub.
- **The pipeline**, at each push: formatting, vet, the tests, the 32
  reference skies in Go and in JavaScript, the site driven in a headless
  Firefox, then the image, then the deployment, and it ends only when the
  site answers `/version` with the commit it built.
- **The watch of the announcement**, once a day, in the cluster
  (`spoke/apps/stellar/watch.yaml` in the deployment repository): the DANE
  records, DNSSEC, the DNS token. A failure is a mail; so is every Monday.

What nothing watches yet: how many requests are served and how many fail.
The server writes no log, on purpose: a line per request would hold the
names visitors ask about.

## Looking at it

```sh
tools/probe https://stellar.mesa.black      # what a visitor would get, now
curl https://stellar.mesa.black/version     # which commit answers
```

With access to the cluster (`ssh -N -L 16443:127.0.0.1:6443 mesa.black`, then
the context `mesa-black`):

```sh
kubectl -n stellar get pods                 # restarts are in the RESTARTS column
kubectl -n stellar top pods                 # memory, against the 64Mi limit
kubectl -n stellar describe pod <name>      # "OOMKilled" under Last State if the limit was hit
kubectl -n stellar get events --sort-by=.lastTimestamp | tail
```

## Going back

A deployment is one commit in the deployment repository, made by the
pipeline: `stellar: deploy <commit>`, which changes the image's tag and
digest in `spoke/apps/stellar/kustomization.yaml`. To go back to the build
before, revert that commit there and push: Argo CD puts the previous image
back within a few minutes, without a new build.

```sh
git log --oneline -5 -- spoke/apps/stellar/kustomization.yaml
git revert <the deploy commit> && git push
```

Then fix `main` here: the next push deploys again.

## What has gone wrong, and what it looks like

- **The pod is killed for memory** (2026-11-24, two minutes down). A square
  picture of a sky took more than the limit. `describe pod` says
  `OOMKilled`. Pictures are now drawn one at a time and `TestPNGMemory`
  bounds each; anything new that allocates per request is measured first
  (`CONTRIBUTING.md`).
- **One answer in a hundred takes ten seconds** (2026-11-27). Many visitors
  asking for one picture drew it one after the other. Found by the load
  test, fixed, kept from coming back by `TestPictureIsDrawnOnce`.
- **A page shows `%!s(MISSING)`** (2026-11-27). A sentence was given fewer
  values than it names. `TestNoSentenceIsBroken` opens every kind of page in
  the three languages, and the probe looks for it too.
- **The pipeline is green and the site is old**. Argo CD had not caught up,
  or the new pod never became ready. The pipeline now waits for `/version`.
- **The log service refuses** (`Cert Spotter` allows a hundred questions an
  hour). The page says so and the rest works; nothing to do but wait.
- **The DNS token ends**. The daily watch mails the next morning; a new
  token goes in with `create-announce-secret.sh`, in the deployment
  repository.

## The load test

Against the image, on a workstation, not against the site:

```sh
make image
podman run -d --name stellar-load -p 127.0.0.1:8097:8080 localhost/stellar serve -addr :8080
# 3,000 requests, 50 at once, over pages, pictures and charts of a few hosts:
xargs -P 50 -n 1 curl -s -o /dev/null -w '%{http_code} %{time_total}\n' < urls.txt
podman stats --no-stream stellar-load
```

On 2026-11-27: 3,000 right answers, 317 a second, the slowest hundredth at
0.44 s, a peak of 28.5 MB.
