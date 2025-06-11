import unittest

from main import Renter

_RENTER = """
{
    "email": "test@example.com",
    "weekly_rent_amt": 260,
    "unix_time_last_paid": 0
}
"""

_RENTER_REAL = """
{
    "email": "test@example.com",
    "weekly_rent_amt": 260,
    "unix_time_last_paid": 1748258028
}
"""

#:


class TestRent(unittest.TestCase):
    def test_calculate_rent_fortnightly(self):
        cur_time = 1209600
        renter = Renter.from_json(_RENTER)
        amt_to_pay = renter.calculate_rent(cur_time)
        self.assertEqual(int(amt_to_pay), 520)

    def test_calculate_rent_real(self):
        cur_time = 1749467628
        renter = Renter.from_json(_RENTER_REAL)
        amt_to_pay = renter.calculate_rent(cur_time)
        self.assertEqual(int(amt_to_pay), 520)

    def test_calculate_rent_monthly(self):
        cur_time = 2419200  # seconds in 28 days
        renter = Renter.from_json(_RENTER)
        amt_to_pay = renter.calculate_rent(cur_time)
        self.assertEqual(int(amt_to_pay), 1040)





if __name__ == "__main__":
    unittest.main()
