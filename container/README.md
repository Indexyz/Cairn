```
docker build . -t cairn-worker-container
```
The agent CLIs (Claude Code, Codex, Pi), the hacker tools from GitHub releases
(katana, nuclei, dalfox, cloudfox, kerbrute) and the Kali packages all resolve to
their latest release at build time, so a rebuild is how you pick up upstream
fixes. Pin a version in the `Dockerfile` if you need byte-reproducible images.

Pass `--build-arg GITHUB_TOKEN=...` to lift the anonymous GitHub API rate limit
used to resolve the release tags; the publish workflow forwards its token.
