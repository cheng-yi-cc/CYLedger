"""Validate Android deployment inputs without the SDK, credentials or network."""
import json
from pathlib import Path
import tempfile
import unittest
from build_android import market_network_bytes


class MarketNetworkConfigTests(unittest.TestCase):
    def check_config(self, config):
        with tempfile.TemporaryDirectory() as folder:
            path = Path(folder) / "config.json"
            path.write_text(json.dumps(config), encoding="utf-8")
            return json.loads(market_network_bytes(path))

    def test_default(self):
        self.assertEqual(json.loads(market_network_bytes(None)), {})

    def test_private_https_and_loopback(self):
        for address in ("https://quotes.example.com", "http://127.0.0.1:8787", "http://[::1]:8787"):
            config = {"relayUrl": address, "relayToken": "a" * 32}
            self.assertEqual(self.check_config(config), config)

    def test_invalid_configuration(self):
        for config in ([], {"apiKey": "forbidden"}, {"relayToken": "a" * 32},
                       {"relayUrl": 3}, {"relayUrl": "https://quotes.example.com", "relayToken": "short"},
                       {"relayUrl": "http://quotes.example.com", "relayToken": "a" * 32},
                       {"relayUrl": "https://user:password@quotes.example.com", "relayToken": "a" * 32},
                       {"relayUrl": "https://quotes.example.com/path", "relayToken": "a" * 32},
                       {"relayUrl": "https://quotes.example.com?token=x", "relayToken": "a" * 32},
                       {"relayUrl": "https://quotes.example.com", "relayToken": "a" * 32 + "\n"}):
            with self.subTest(config=config):
                with self.assertRaises(ValueError):
                    self.check_config(config)


if __name__ == "__main__":
    unittest.main()
