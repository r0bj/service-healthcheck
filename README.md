# service-healthcheck

Restarts upstart service when healthcheck defined as HTTP URL fails.

```
usage: service-healthcheck --service=SERVICE [<flags>]

Flags:
      --help              Show context-sensitive help (also try --help-long and --help-man).
  -u, --url="http://127.0.0.1:8080"
                          health check URL
  -s, --service=SERVICE   local service name
  -t, --timeout=3         timeout for HTTP requests in seconds
  -i, --interval=5        healthcheck interval
      --fail-interval=2   healthcheck interval during failed period
      --fail-threshold=3  failed healthchecks threshold
      --initial-delay=10  initial delay in seconds
  -n, --dry-run           dry run
      --version           Show application version.
```
