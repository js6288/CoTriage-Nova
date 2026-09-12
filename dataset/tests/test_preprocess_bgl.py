import sys
import unittest
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

from preprocess_bgl import normalize_message, parse_bgl_line, split_for_template, template_id


class BGLPreprocessTest(unittest.TestCase):
    def test_parse_alert_line(self):
        line = (
            "APPREAD 1117869872 2005.06.04 R23-M1-N8-I:J18-U11 "
            "2005-06-04-00.24.32.398284 R23-M1-N8-I:J18-U11 RAS APP FATAL "
            "ciod: failed to read from 172.16.96.116:33399\n"
        )
        record = parse_bgl_line(line)
        self.assertIsNotNone(record)
        self.assertTrue(record["is_alert"])
        self.assertEqual(record["raw_label"], "APPREAD")
        self.assertEqual(record["component"], "APP")
        self.assertEqual(record["level"], "FATAL")
        self.assertIn("<IP>:<NUM>", record["template_text"])

    def test_parse_non_alert_line(self):
        line = (
            "- 1117838570 2005.06.03 R02-M1-N0-C:J12-U11 "
            "2005-06-03-15.42.50.363779 R02-M1-N0-C:J12-U11 RAS KERNEL INFO "
            "instruction cache parity error corrected\n"
        )
        record = parse_bgl_line(line)
        self.assertIsNotNone(record)
        self.assertFalse(record["is_alert"])

    def test_template_id_and_split_are_stable(self):
        template = normalize_message("connection from 10.0.0.1:3306 failed")
        self.assertEqual(template_id(template), template_id(template))
        self.assertEqual(
            split_for_template(template_id(template), "seed", 0.7, 0.15),
            split_for_template(template_id(template), "seed", 0.7, 0.15),
        )

    def test_normalizes_bare_hex_and_colon_hex_sequence(self):
        message = "iar 003a929c dear 012982b8 chip FF:F2:9F:16:E2:37:00:0D:60:E9:1D:C8"
        normalized = normalize_message(message)
        self.assertEqual(normalized, "iar <HEX> dear <HEX> chip <HEXSEQ>")

    def test_normalizes_alphanumeric_device_identifiers(self):
        message = "Lustre mount FAILED : bglio817 on R73-M1-N4 eth0"
        normalized = normalize_message(message)
        self.assertEqual(
            normalized,
            "Lustre mount FAILED : bglio<NUM> on R<NUM>-M<NUM>-N<NUM> eth<NUM>",
        )

    def test_rejects_malformed_line(self):
        self.assertIsNone(parse_bgl_line("not enough fields\n"))


if __name__ == "__main__":
    unittest.main()
