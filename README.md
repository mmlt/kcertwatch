kcertwatch keeps track of TLS certificates in Kubernetes Secrets and publishes the NotAfter time as a Prometheus metric.

Supported Secret types:
- Opaque
- kubernetes.io/tls

Supported certificate formats:
- PEM


## Run
Flags and environment variables when testing with `kubectl proxy`:
    --k8s-api=http://localhost:8001  
    --logtostderr=true --vmodule=client=9 --v=2


-v=2 cert add/delete
-v=6 call logging
```
Query min(kcertwatch_cert_expire_time_seconds{namespace=~"$namespace"}) by (name) - time()
```

## Development
See Makefile

### tools
`github.com/itchio/gothub` is used for releasing binaries to github.

`github.com/golangci/golangci-lint` is used for linting.
`curl -sfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh| sh -s v1.17.1`

## TODO
- Show CN of cert?
- Should Secret type=Opaque be supported? maybe gated by flag?