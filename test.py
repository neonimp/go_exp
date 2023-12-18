"""
Quick tester for the bridge, sending example data to the bridge
"""

import smtplib
import json
from email.mime.text import MIMEText


if __name__ == "__main__":
    with open("test.json", "r", encoding="utf-8") as f:
        d = json.load(f)
        sender = d["From"]
        to = d["To"]

    # Create a text/plain message
    msg = MIMEText("Test message from python")
    msg['Subject'] = 'Test message'
    msg['From'] = sender
    msg['To'] = to
    with smtplib.SMTP('localhost', 1025) as s:
        s.user = "test"
        s.password = "test"
        s.auth_plain()
        s.login("test", "test")
        s.sendmail(msg['From'], [msg['To']], msg.as_string())
