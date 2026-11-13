# lab (history)

A design experiment from 2026-09-30, frosted glass on the feed, the diary and the entry page. It was dropped on 2026-10-10 and is not the current design: the app uses plain light cards with no glass or blur. No committed app code imports it; it stays only as a record, and `docs/frosted-editorial-experiment.md` says what was tried.

`bash lab/enable.sh` adds route shims under `app/lab/` to open it in a development build, and `bash lab/disable.sh` removes them. Never commit `app/lab/`.
