import os
import re
from concurrent import futures

import grpc

import nlp_pb2


EMBEDDING_MODEL = os.getenv("EMBEDDING_MODEL", "all-MiniLM-L6-v2")
EMBEDDING_DIM = int(os.getenv("EMBEDDING_DIM", "384"))
PIPELINE_VERSION = os.getenv("PIPELINE_VERSION", "v0")

MAX_MESSAGE_BYTES = 256 * 1024 * 1024

WORD_OR_PUNCT_PATTERN = re.compile(r"\w+|[^\w\s]", re.UNICODE)


class NLPService:
    def Health(self, _request, _context):
        return nlp_pb2.HealthResponse(
            service="nlp-python",
            status="ready",
            pipeline_version=PIPELINE_VERSION,
            embedding_model=EMBEDDING_MODEL,
            embedding_dim=EMBEDDING_DIM,
        )

    def AnalyzeDocument(self, request, _context):
        stopwords = {word.strip().lower() for word in request.custom_stopwords if word.strip()}
        sentence_languages = []
        tokens = []

        default_language = request.language_hint.strip() if request.language_hint else "unknown"
        confidence = 0.9 if default_language != "unknown" else 0.5

        for passage in request.passages:
            for sentence in passage.sentences:
                sentence_languages.append(
                    nlp_pb2.SentenceLanguage(
                        sentence_id=sentence.sentence_id,
                        language=default_language,
                        confidence=confidence,
                    )
                )

                for token_index, match in enumerate(WORD_OR_PUNCT_PATTERN.finditer(sentence.text)):
                    surface = match.group(0)
                    lemma = surface.lower()
                    is_punct = not any(char.isalnum() for char in surface)
                    tokens.append(
                        nlp_pb2.Token(
                            sentence_id=sentence.sentence_id,
                            token_index=token_index,
                            surface=surface,
                            lemma=lemma,
                            pos="PUNCT" if is_punct else "X",
                            is_stopword=(lemma in stopwords),
                            is_punct=is_punct,
                            start_char=sentence.start_char + match.start(),
                            end_char=sentence.start_char + match.end(),
                        )
                    )

        return nlp_pb2.AnalyzeDocumentResponse(
            pipeline_version=PIPELINE_VERSION,
            sentence_languages=sentence_languages,
            tokens=tokens,
            mentions=[],
        )

    def EmbedPassages(self, request, _context):
        return nlp_pb2.EmbedPassagesResponse(
            model_name=EMBEDDING_MODEL,
            dim=EMBEDDING_DIM,
            embeddings=[
                nlp_pb2.PassageEmbedding(
                    passage_id=passage.passage_id,
                    values=[0.0] * EMBEDDING_DIM,
                )
                for passage in request.passages
            ],
        )


def _unary_unary(handler_method, request_type, response_type):
    return grpc.unary_unary_rpc_method_handler(
        handler_method,
        request_deserializer=request_type.FromString,
        response_serializer=response_type.SerializeToString,
    )


def main():
    port = int(os.getenv("NLP_GRPC_PORT", "50051"))
    service = NLPService()
    grpc_server = grpc.server(
        futures.ThreadPoolExecutor(max_workers=4),
        options=[
            ("grpc.max_receive_message_length", MAX_MESSAGE_BYTES),
            ("grpc.max_send_message_length", MAX_MESSAGE_BYTES),
        ],
    )
    grpc_server.add_generic_rpc_handlers(
        (
            grpc.method_handlers_generic_handler(
                "concordance.nlp.v1.NLPService",
                {
                    "Health": _unary_unary(service.Health, nlp_pb2.HealthRequest, nlp_pb2.HealthResponse),
                    "AnalyzeDocument": _unary_unary(
                        service.AnalyzeDocument,
                        nlp_pb2.AnalyzeDocumentRequest,
                        nlp_pb2.AnalyzeDocumentResponse,
                    ),
                    "EmbedPassages": _unary_unary(
                        service.EmbedPassages,
                        nlp_pb2.EmbedPassagesRequest,
                        nlp_pb2.EmbedPassagesResponse,
                    ),
                },
            ),
        )
    )

    grpc_server.add_insecure_port(f"127.0.0.1:{port}")
    grpc_server.start()
    print(f"nlp-python gRPC listening on :{port}")
    grpc_server.wait_for_termination()


if __name__ == "__main__":
    main()
