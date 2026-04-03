EMBEDDING_MODEL = "all-MiniLM-L6-v2"
EMBEDDING_DIM = 384


def model_metadata() -> dict:
    return {
        "model": EMBEDDING_MODEL,
        "dim": EMBEDDING_DIM,
    }
