# Use cases

Popular Compose stacks that double as bianpai's real-world test corpus. Each stack must
parse and load — `TestUsecaseStacksLoad` walks this folder in `make check`, so a broken
stack fails CI. `make smoke` additionally verifies supported Apple-container stacks with
a fake `container` CLI and asserts DNS-dependent stacks fail with the expected limitation
message. Images are pulled through `mirror.gcr.io` where the mirror has a usable copy.
Run `make smoke-real` to use the real Apple `container` CLI. Host-published app
endpoints are verified with `curl`; stacks that require container-to-container DNS must
fail early with a clear Apple-container limitation message.

| Stack | Exercises |
| --- | --- |
| [`caddy-host`](caddy-host) | single-service host-published Caddy file server for Apple container real smoke |
| [`multi-host`](multi-host) | multiple network-isolated containers (Caddy + Python) running side-by-side on Apple container backend |
| [`etcd-single`](etcd-single) | single-node host-published etcd for Apple container real smoke |
| [`python-valkey-caddy`](python-valkey-caddy) | build-free app + cache + reverse proxy; named volume, config bind mount, `depends_on`, service-name DNS |
| [`etcd-cluster`](etcd-cluster) | 3-node cluster discovering peers by service name; map-form `environment`, one published client port per node |
| [`postgres-host`](postgres-host) | single-service host-published Postgres for Apple container real smoke |
| [`postgres-adminer`](postgres-adminer) | database + admin UI; named volume, `depends_on` |
| [`python-host`](python-host) | single-service host-published app access for Apple container real smoke |

Run a host-published single-service stack with the Apple container backend:

```sh
cd usecases/python-host
bianpai --backend container up
```

The multi-service stacks remain in the corpus for parser/argv coverage, but Apple
`container` rejects them by default because they require Compose service-name DNS.
