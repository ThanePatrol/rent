import datetime
import json
import smtplib
import os
import argparse
from email.message import EmailMessage

import logging

logger = logging.getLogger(__name__)

SECONDS_IN_WEEK = 7 * 24 * 60 * 60


class Renter:
    email: str
    weekly_rent_amt: int
    rent_in_seconds: float
    unix_time_last_paid: int

    def __init__(self, email: str, rent: int, last_pay: int):
        self.email = email
        self.weekly_rent_amt = rent
        self.rent_in_seconds = rent / SECONDS_IN_WEEK
        self.unix_time_last_paid = last_pay

    def __str__(self) -> str:
        return f"renter(email={self.email}, amt={self.weekly_rent_amt})"

    def to_json(self):
        return json.dumps(self, default=lambda o: o.__dict__, sort_keys=True, indent=4)

    def calculate_rent(self, cur_time: int) -> float:
        seconds_since_last_pay = cur_time - self.unix_time_last_paid
        amount_to_pay = self.rent_in_seconds * seconds_since_last_pay
        return amount_to_pay

    @staticmethod
    def from_json(json_str: str):
        data = json.loads(json_str)
        return Renter(
            data["email"], data["weekly_rent_amt"], data["unix_time_last_paid"]
        )


class Config:
    def __init__(self, password: str, email: str, bsb: str, account: str):
        self.password = password
        self.email = email
        self.bsb = bsb
        self.account = account

    def __str__(self):
        return f"Config(email={self.email}, bsb={self.bsb}, account={self.account} pass={self.password})"

    @staticmethod
    def from_env():

        password = os.environ.get("PASSWORD")

        email = os.environ.get("EMAIL")
        bsb = os.environ.get("BSB")
        account = os.environ.get("ACCOUNT")

        assert isinstance(email, str)
        assert isinstance(password, str)
        assert isinstance(bsb, str)
        assert isinstance(account, str)
        logger.info(f"email={email} bsb={bsb} account={account}")

        return Config(password=password, email=email, bsb=bsb, account=account)


def send_email(config: Config, renter: Renter, amount_to_pay: float):

    msg = EmailMessage()
    msg["Subject"] = "Rent Notice"
    msg["From"] = config.email
    msg["To"] = ", ".join([renter.email, config.email])
    msg.set_content(
        f"""\
        Amount owing: {amount_to_pay:.2f}

        BSB: {config.bsb}
        Account: {config.account}

        Please contact me within 24 hours if there are any issues!

        Best,
    """
    )

    s = smtplib.SMTP("smtp.gmail.com", 587)
    s.starttls()
    s.login(config.email, config.password)
    # s = smtplib.SMTP("smtp-mail.outlook.com", 587)
    # s.starttls()
    # s.login(config.email, config.password)

    logger.info(msg.get_content())
    s.send_message(msg)
    s.quit()
    logger.info("sent email 😋")


def main():
    logging.basicConfig(
        filename="/var/log/rent/rent.log",
        level=logging.INFO,
        format="%(asctime)s :: %(levelname)s :: %(name)s:%(lineno)d :: %(message)s",
    )

    config = Config.from_env()
    logger.info(f"config= {config}")

    logger.info("about to parse args")

    parser = argparse.ArgumentParser(
        prog="rent-memes",
        description="What the program does",
        epilog="Text at the bottom of help",
    )

    parser.add_argument("-e", "--email")
    args = parser.parse_args()

    logger.info("parsed args")

    file_name = f"{args.email}.json"

    renter = Renter("", 0, 0)

    logger.info("about to open file")
    with open(file_name, "r") as f:
        as_str = "".join(f.readlines())
        renter = Renter.from_json(as_str)
        logger.info(f"parsed file and got={renter}")

    logger.info("about to send email")
    cur_time = int(datetime.datetime.now(datetime.timezone.utc).timestamp())
    amount_to_pay = renter.calculate_rent(cur_time)

    send_email(config, renter, amount_to_pay)
    logger.info("sent email")
    # Once we have sent an email the time they last paid is now
    renter.unix_time_last_paid = cur_time

    with open(f"{args.email}.json", "w") as f:
        f.write(renter.to_json())
    logger.info("DONE")


if __name__ == "__main__":
    main()
