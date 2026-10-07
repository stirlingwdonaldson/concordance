export type GlossaryEntry = {
  term: string;
  short: string;
  example?: string;
};

/** Plain-language definitions shown on hover and in the glossary panel. */
export const GLOSSARY = {
  concordance: {
    term: "Concordance",
    short:
      "An index of every word in a text, with how often it appears and where.",
    example: "Like the index at the back of a book, but for every word.",
  },
  kwic: {
    term: "KWIC",
    short:
      "Key Word In Context. Shows each place a word appears with the words around it, so you can see how it's used.",
    example: "…the model that learns from data improves…",
  },
  lemma: {
    term: "Lemma",
    short:
      "The dictionary form of a word. Grouping by lemma counts different forms together.",
    example: "run, runs, running and ran all count as “run”.",
  },
  token: {
    term: "Token",
    short: "One word or number in the text. Punctuation is not counted here.",
    example: "“The cat sat.” has 3 tokens.",
  },
  types: {
    term: "Unique terms",
    short:
      "How many different lemmas appear. Repeated words are only counted once.",
    example: "“the cat saw the cat” has 5 tokens but 3 unique terms.",
  },
  hapax: {
    term: "Hapax legomenon",
    short:
      "A word that appears exactly once in the text. Often names, rare words or typos.",
    example: "In a textbook, a one-off author name is usually a hapax.",
  },
  ttr: {
    term: "Vocabulary variety",
    short:
      "Unique terms divided by total words. Higher means a more varied vocabulary. Longer texts always score lower, so only compare similar lengths.",
    example: "0.05 means about 1 new term per 20 words.",
  },
  frequency: {
    term: "Frequency",
    short: "How many times a word appears in the text.",
  },
  pos: {
    term: "Part of speech",
    short: "The grammatical role of a word, such as noun, verb or adjective.",
  },
  sentence: {
    term: "Sentence",
    short: "A unit of text ending in a full stop, question mark or similar.",
  },
  passage: {
    term: "Passage",
    short:
      "A chunk of consecutive sentences, usually a paragraph. The text is analysed passage by passage.",
  },
  pipeline: {
    term: "Processing steps",
    short:
      "The steps each file goes through before you can explore it: read, split, tag, then index.",
  },
  keyness: {
    term: "Key words",
    short:
      "Words used much more in one file than the other. They show what each file is about that the other isn't.",
    example:
      "Comparing a textbook with a licence: “agent” stands out in the textbook, “license” in the licence.",
  },
  evidence: {
    term: "Evidence",
    short:
      "How unlikely the difference is to be chance (a log-likelihood score). Strong means very unlikely, Moderate somewhat unlikely, Weak could easily be chance.",
  },
  perMillion: {
    term: "Per million words",
    short:
      "How often a word appears for every million words, so files of different lengths can be compared fairly.",
    example: "50 per million is about once every 20,000 words.",
  },
  wildcard: {
    term: "Wildcard (*)",
    short:
      "A star stands for any letters. Searching learn* finds learn, learns, learning and learned.",
  },
  context: {
    term: "Context",
    short: "The words immediately before and after the highlighted word.",
  },
} satisfies Record<string, GlossaryEntry>;

export type GlossaryKey = keyof typeof GLOSSARY;

export const SUPPORTED_FORMATS = [
  "PDF",
  "DOCX",
  "EPUB",
  "ODT",
  "HTML",
  "TXT",
  "MD",
];

export const EXAMPLE_PROJECT_NAMES = [
  "Linguistics 101",
  "Thesis reading list",
  "Annual reports",
];

export const EXAMPLE_SEARCHES = ["learning", "model", "learn*"];
