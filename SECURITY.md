# Security Policy

## Reporting a Vulnerability

If you discover a security vulnerability in this project, please report it **privately** so that it can be addressed responsibly.

**Do not open a public GitHub issue for security vulnerabilities.**

### Contact

Send vulnerability reports to:

**Davide Guerri**  
Email: davide.guerri@gmail.com

If possible, include the following information:

- Description of the vulnerability
- Steps to reproduce the issue
- A proof-of-concept exploit (if available)
- The affected version(s) (or affected sha commits)
- Potential impact
- Suggested remediation (if known)

---

## Encrypted Reports (PGP)

Vulnerability reports may be encrypted using the following OpenPGP key.

**Owner:** Davide Guerri  
**Primary email:** davide.guerri@gmail.com  
**Key ID:** `4D97F757E8074EC8`

**Fingerprint**

```
0692 AD3F 13A5 016A 3ACF  7245 4D97 F757 E807 4EC8
```

**Key details**

```
pub   rsa4096 2022-09-28 [SC] [expires: 2029-09-29]
      0692AD3F13A5016A3ACF72454D97F757E8074EC8
uid           Davide Guerri <davide.guerri@gmail.com>
uid           Davide Guerri <davide@guerri.me>
uid           Davide Guerri <dguerri@google.com>
sub   rsa4096 2022-09-28 [S] [expires: 2026-09-27]
sub   rsa4096 2022-09-28 [E] [expires: 2026-09-27]
sub   rsa4096 2022-09-28 [A] [expires: 2026-09-27]
```

The public key can be retrieved from common OpenPGP keyservers.

Example:

```
gpg --recv-keys 4D97F757E8074EC8
```

---

## Response Timeline

While response times may vary depending on availability and complexity:

- **Acknowledgement:** typically within 72 hours  
- **Initial triage:** within 7 days

If the issue is confirmed as a valid vulnerability, a fix will be developed and released as soon as reasonably possible.

---

## Responsible Disclosure

This project follows a responsible disclosure process:

1. The vulnerability is reported privately.
2. The issue is verified and a fix is prepared.
3. A patched version is released.
4. The vulnerability may then be publicly disclosed.

Security researchers who report valid vulnerabilities may be credited in release notes or documentation unless they request anonymity.

---

## Scope

This policy applies only to vulnerabilities within this repository.

Issues affecting third-party dependencies should be reported to the respective upstream maintainers.
