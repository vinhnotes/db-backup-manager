# CodeQL review: notification email

The `go/email-injection` finding at `sendMail`'s SMTP DATA write is a false
positive for the current implementation. Do not disable the rule or exclude
`notify.go` from analysis. If the message format or validation changes, review
this conclusion again.

## Evidence

CodeQL CLI 2.27.4 with `codeql/go-queries` 1.6.14 identifies two input paths:

1. The admin's notification sender address goes through `mail.ParseAddress`
   and `mail.Address.String` before serialization into the From header.
2. A requested restore database name becomes a value in the notification body,
   which is sent as MIME quoted-printable `text/plain; charset=UTF-8`.

The full Go `security-and-quality` suite (57 rules) was run against the revised
source: it reports this one email finding, zero `go/path-injection` findings
and zero `go/request-forgery` findings. Path access now goes through an
operator-configured mount allowlist; HTTP requests use an independently
configured origin allowlist. No rule exclusions or custom sanitizer models
were added.

`mailMessage` rejects CR, LF and NUL in sender, recipient and subject inputs
before opening an SMTP connection. Sender/recipient parse errors are handled.
The body is separated from the headers by a fixed blank line and MIME encoded;
SMTP DATA uses Go's dot-stuffing writer. No HTML email or password reset link
is constructed from these inputs.

Regression tests in `notification_security_test.go` cover header injection,
invalid addresses, MIME round trips, and a complete SMTP transaction containing
a fake DATA terminator, Bcc header, Content-Type header and script markup. The
fake headers remain in the body, only the configured recipient is used, and
the content type stays text/plain.

The upstream [email injection configuration](https://github.com/github/codeql/blob/main/go/ql/src/Security/CWE-640/EmailInjection.qll)
tracks all input-to-email flows without a sanitizer/barrier predicate, so
successful MIME encoding and validation do not stop this result.

## Suggested dismissal

Dismiss the email finding as **False positive**, using this explanation:

> Reviewed the complete CodeQL paths: the sender is parsed and serialized with
> net/mail after CR/LF/NUL rejection; the restore database name appears only in
> a quoted-printable text/plain MIME body. An SMTP wire regression test confirms
> that injected Bcc/Content-Type headers and DATA terminators remain body text
> and cannot add recipients or change the message type. No untrusted reset URL
> is generated. The query does not model these validation/encoding boundaries.

The dismissal is specific to this email construction. It does not apply to
future HTML emails, URL construction or other mail senders.
