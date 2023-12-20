# Bridge SMTP only software to use API-based email sending.
As the title says, run this command to host a simple SMTP server locally. point any software that only supports SMTP to it,
any email sent to this server will be sent with the configured email api provider by the bridge.

# TODOs
1. Implement providers other than SES;
2. Improve the code organization and testability;

# Quick manual
Build with `make` then run as follows:
`bin/smtpbridge -c example.toml`, with this config it will be listening on port 1025.
