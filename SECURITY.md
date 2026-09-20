# Security

Report vulnerabilities privately to the maintainers of the `gotthboard`
organization. Do not include GoDaddy PATs, local service tokens, production
zones, record contents, or deployment credentials in public issues.

The extension accepts only protected local runtime files and authenticated
Unix-socket RPC. It has no registrar, nameserver-delegation, domain-lifecycle,
Docker-socket, shell, plugin, or generic HTTP authority. Suspected secret
disclosure, local-authentication bypass, redirect following, arbitrary request
construction, mutation retry, false reconciliation, or artifact substitution
is release-blocking.
