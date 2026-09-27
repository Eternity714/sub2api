# GRS.AI Async contract probe

The probe is offline by default and is not part of CI. It sends exactly one
`replyType=async` generation request only with `--live`, then polls `/result`.
It reports status and result count, never the task ID, image URLs, prompt,
response body, or API key. It does not test Sub2API billing or local S3 storage.

```powershell
python test/grsai_live_contract.py --self-test
python test/grsai_live_contract.py
```

For an authorized live check, set `GRSAI_BASE` (HTTPS international endpoint)
and `GRSAI_KEY` in the local process environment without putting credentials
in the repository or command history, then run:

```powershell
python test/grsai_live_contract.py --live --model <approved-model>
```

The live call may incur provider charges. The script limits itself to one
generation request and a 30-minute polling window; it does not enforce a
monetary spending cap. A successful provider probe must still be followed by
an authenticated Sub2API end-to-end check of local task IDs, S3 URLs, and
billing before release.
