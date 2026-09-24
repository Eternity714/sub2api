# GRS.AI live contract probe

This script uses only Python's standard library. `python test/grsai_live_contract.py --self-test`
is offline and is the only invocation used by CI. Running without flags exits 2 before
opening a connection. No credentials are required for the self-tests.

## Separate operator-controlled run

Do not run live during routine testing. Set `GRSAI_BASE` to
`https://grsaiapi.com` (the provider's global node)
and `GRSAI_KEY` to a dedicated key through your secret-management process. Do not
put the key in shell history or CI. Then run:

```text
python test/grsai_live_contract.py --live
```

If the local Python proxy path is unreliable, verify connectivity with a read-only
request first and set `NO_PROXY=grsaiapi.com` for this invocation only. Do not
repeat a failed generation request without checking upstream task activity.

The `--live` flag must match exactly; HTTPS is restricted to that provider host,
redirects are refused, and the process starts at most one POST to `/v1/api/generate`
with `model=nano-banana-2-lite` and `replyType=stream`. It checks SSE frames and
then queries `/v1/api/result?id=...` with a total 180-second deadline. A transport
failure after opening the POST may still represent a submitted task: **do not rerun
automatically**. Review upstream activity and charges manually before any retry.

This probe validates the upstream protocol, not upstream costs. Exit 0 means the
stream and result query matched; exit 1 denotes failed protocol/transport, and exit 2
denotes input rejection. A successful probe does not validate local billing.
Output omits the key, full task ID, prompt, image URLs and raw responses. Do not
redirect raw upstream traffic or enable HTTP wire logging for this run.

The release gate stays **no-go** until the live protocol passes repeatedly and
the remaining functional and billing checks pass. Roll out JSON, then Async, then Stream; opening
Stream also requires successful bound-ID disconnect recovery, no duplicate settlement,
and no abnormal `manual_review` backlog. Production traffic changes must follow
the repository's gray-release procedure.
