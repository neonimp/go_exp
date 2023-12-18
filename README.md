# Bridge SMTP only software to use SES API
As the title says, run this command to host a simple SMTP server locally. Any email sent to it will be then sent using SES.

# TODOs
1. Implement providers other than SES;
2. Improve the code organization and testability;

# Quick manual
Build with `make` then run as follows:
`bin/smtpbridge -c example.toml`, with this config it will be listening on port 1025.
