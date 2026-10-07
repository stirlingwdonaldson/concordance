import os
import sys
import unittest

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", "server"))

from analyzer import CHUNK_CHARS, RegexAnalyzer, SpacyAnalyzer, _chunks  # noqa: E402


def load_spacy():
    try:
        return SpacyAnalyzer.load("en_core_web_sm")
    except Exception:  # noqa: BLE001
        return None


class RegexAnalyzerTest(unittest.TestCase):
    def test_offsets_point_back_into_the_text(self):
        text = "Hello, world 42!"
        for tok in RegexAnalyzer().analyze([text])[0]:
            self.assertEqual(text[tok.start : tok.end], tok.surface)

    def test_punctuation_is_flagged(self):
        toks = RegexAnalyzer().analyze(["a, b"])[0]
        self.assertEqual([t.is_punct for t in toks], [False, True, False])


class ChunkingTest(unittest.TestCase):
    def test_long_text_is_split_without_losing_characters(self):
        text = ("word " * (CHUNK_CHARS // 2))
        pieces = list(_chunks(text))
        self.assertGreater(len(pieces), 1)
        self.assertEqual("".join(piece for _, piece in pieces), text)
        for offset, piece in pieces:
            self.assertEqual(text[offset : offset + len(piece)], piece)
            self.assertLessEqual(len(piece), CHUNK_CHARS)


@unittest.skipIf(load_spacy() is None, "spaCy model not installed")
class SpacyAnalyzerTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.analyzer = load_spacy()

    def test_lemmas_group_word_forms_and_pos_is_real(self):
        toks = self.analyzer.analyze(["The dogs were running."])[0]
        by_surface = {t.surface: t for t in toks}
        self.assertEqual(by_surface["dogs"].lemma, "dog")
        self.assertEqual(by_surface["dogs"].pos, "NOUN")
        self.assertEqual(by_surface["running"].lemma, "run")
        self.assertEqual(by_surface["running"].pos, "VERB")
        self.assertTrue(by_surface["."].is_punct)

    def test_offsets_point_back_into_the_text(self):
        text = "It doesn't matter,  really."  # contraction and a double space
        toks = self.analyzer.analyze([text])[0]
        for tok in toks:
            self.assertEqual(text[tok.start : tok.end], tok.surface)
        self.assertFalse(any(t.surface.strip() == "" for t in toks))

    def test_one_result_list_per_input_even_when_empty(self):
        result = self.analyzer.analyze(["", "Hi there.", "   "])
        self.assertEqual(len(result), 3)
        self.assertEqual(result[0], [])

    def test_oversized_sentence_does_not_fail(self):
        text = "alpha " * (CHUNK_CHARS)  # 600k chars, no punctuation
        toks = self.analyzer.analyze([text])[0]
        self.assertEqual(len(toks), CHUNK_CHARS)
        self.assertEqual(text[toks[-1].start : toks[-1].end], "alpha")


if __name__ == "__main__":
    unittest.main()
