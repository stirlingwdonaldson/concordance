import unittest

from pipelines import embeddings


class EmbeddingsTest(unittest.TestCase):
    def test_model_metadata(self):
        metadata = embeddings.model_metadata()
        self.assertEqual(metadata["model"], "all-MiniLM-L6-v2")
        self.assertEqual(metadata["dim"], 384)


if __name__ == "__main__":
    unittest.main()
