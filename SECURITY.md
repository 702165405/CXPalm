# Security And Privacy

CXPalm exposes privileged access to an official desktop session. Treat the
gateway, desktop account and trusted HTTPS reverse proxy as security-sensitive.
This is an independent project, not an OpenAI product or a hardened service
with a security certification.

## Deployment Boundaries

- Bind the gateway and reverse tunnel to loopback only. Use a fixed HTTPS
  origin and preserve Host/Origin validation and device authorization.
- IP allowlisting behind a tunnel requires the proxy to overwrite
  `X-CXPalm-Client-IP` with the connection source address before enabling
  `trustedProxyIP`. Do not trust arbitrary client forwarding headers.
- Revoke lost devices. Changing the local login credentials revokes sessions.
- Never expose databases, uploads, logs, backups or arbitrary local files as
  public downloads. Do not put tokens in URLs or commit signing keys.
- Keep original-owner execution and uncertain-write non-replay checks intact.
- Review the compiled native update allowlists before distributing a custom
  build; its login server and update origin are independent trust decisions.

## Reporting A Vulnerability

Do not open a public issue with exploit details or sensitive logs. No private
reporting endpoint is configured yet. Before public launch the maintainer must
enable GitHub private vulnerability reporting or publish a verified private
contact. Until then, request a private reporting channel without posting the
vulnerability itself. No response-time or supported-version SLA is promised.

## Source Hygiene

Only source, synthetic fixtures, public assets and dependency lockfiles belong
in Git. `node scripts/check-repository.cjs` checks selected risky names, file
types and credential patterns without printing matching values. It is not a
complete secret scanner. Review manually as well. If a secret was published,
revoke/rotate it; deleting its latest file alone is not sufficient.

Git author/committer names and email addresses are public metadata, separate
from file contents. Use GitHub's verified noreply address for new commits if
you do not want a personal address exposed. A normal cleanup commit cannot
remove existing metadata or historical deployment identifiers from old commits,
tags, releases, clones or caches. History rewriting requires coordinated,
explicit approval and is not performed automatically by release scripts.

Current-source checks are bounded heuristic/manual checks, not proof that all
identifying information is absent from every binary, fork, cache or third-party
copy. Local backups and previous installations are not public source artifacts.
