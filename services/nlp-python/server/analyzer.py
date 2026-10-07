"""Turns sentences into tokens with lemma and part of speech.

SpacyAnalyzer is the real thing. RegexAnalyzer is the old fallback: it still
produces a usable concordance (every word is its own lemma, no grammar) and is
used when spaCy or its English model isn't installed, or fails on a batch.
"""

import os
import re
from dataclasses import dataclass
from typing import Iterable, Iterator, List, Sequence

# spaCy refuses texts over nlp.max_length (1M chars). Real sentences are short,
# but a PDF with no punctuation can produce a monster, so long text is chunked.
CHUNK_CHARS = 100_000

WORD_OR_PUNCT_PATTERN = re.compile(r"\w+|[^\w\s]", re.UNICODE)


@dataclass(frozen=True)
class Tok:
    surface: str
    lemma: str
    pos: str
    is_punct: bool
    is_stop: bool
    start: int  # offsets relative to the sentence text
    end: int


def _is_punct(surface: str) -> bool:
    return not any(char.isalnum() for char in surface)


def _chunks(text: str) -> Iterator[tuple[int, str]]:
    """Yield (offset, piece) so no piece exceeds CHUNK_CHARS, splitting on whitespace."""
    if len(text) <= CHUNK_CHARS:
        yield 0, text
        return
    start = 0
    while start < len(text):
        end = min(start + CHUNK_CHARS, len(text))
        if end < len(text):
            split = text.rfind(" ", start, end)
            if split > start:
                end = split + 1
        yield start, text[start:end]
        start = end


class RegexAnalyzer:
    name = "regex-fallback"

    def analyze(self, texts: Sequence[str]) -> List[List[Tok]]:
        out: List[List[Tok]] = []
        for text in texts:
            toks = []
            for match in WORD_OR_PUNCT_PATTERN.finditer(text):
                surface = match.group(0)
                punct = _is_punct(surface)
                toks.append(
                    Tok(
                        surface=surface,
                        lemma=surface.lower(),
                        pos="PUNCT" if punct else "X",
                        is_punct=punct,
                        is_stop=False,
                        start=match.start(),
                        end=match.end(),
                    )
                )
            out.append(toks)
        return out


class SpacyAnalyzer:
    def __init__(self, nlp, model_name: str):
        self._nlp = nlp
        self._fallback = RegexAnalyzer()
        self.name = f"spacy-{model_name}-{nlp.meta.get('version', 'unknown')}"

    @classmethod
    def load(cls, model_name: str) -> "SpacyAnalyzer":
        import spacy  # imported lazily so a missing install only disables spaCy

        # We only need tagging and lemmas, so skip the slow parser and NER.
        nlp = spacy.load(model_name, disable=["parser", "ner"])
        nlp.max_length = CHUNK_CHARS * 2
        return cls(nlp, model_name)

    def analyze(self, texts: Sequence[str]) -> List[List[Tok]]:
        # Flatten into chunks so one huge sentence can't fail the whole batch.
        pieces: List[tuple[int, int, str]] = []  # (text index, offset, piece)
        for index, text in enumerate(texts):
            for offset, piece in _chunks(text):
                pieces.append((index, offset, piece))

        out: List[List[Tok]] = [[] for _ in texts]
        try:
            docs = list(self._nlp.pipe((piece for _, _, piece in pieces), batch_size=256))
        except Exception as error:  # noqa: BLE001 - any spaCy failure must not lose the document
            print(f"spaCy failed on a batch ({error!r}); using the fallback tokenizer for it")
            return self._fallback.analyze(texts)

        for (index, offset, _), doc in zip(pieces, docs):
            out[index].extend(self._tokens(doc, offset))
        return out

    @staticmethod
    def _tokens(doc, offset: int) -> Iterable[Tok]:
        for token in doc:
            if token.is_space:
                continue
            surface = token.text
            lemma = token.lemma_.strip().lower()
            if not lemma or any(char.isspace() for char in lemma):
                lemma = surface.lower()
            punct = token.is_punct or _is_punct(surface)
            yield Tok(
                surface=surface,
                lemma=lemma,
                pos="PUNCT" if punct else (token.pos_ or "X"),
                is_punct=punct,
                is_stop=bool(token.is_stop),
                start=offset + token.idx,
                end=offset + token.idx + len(surface),
            )


def load_analyzer():
    """Best available analyzer. Never raises: the app must work without spaCy."""
    if os.getenv("NLP_DISABLE_SPACY") == "1":
        print("spaCy disabled by NLP_DISABLE_SPACY; using the regex tokenizer")
        return RegexAnalyzer()
    model = os.getenv("SPACY_MODEL", "en_core_web_sm")
    try:
        analyzer = SpacyAnalyzer.load(model)
        print(f"loaded {analyzer.name}")
        return analyzer
    except Exception as error:  # noqa: BLE001
        print(
            f"spaCy model {model!r} unavailable ({error!r}); using the regex tokenizer. "
            "Word forms will not be grouped and parts of speech will be unknown."
        )
        return RegexAnalyzer()
