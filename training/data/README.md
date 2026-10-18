# Phase 2 synthetic smoke data

The two task datasets are project-authored synthetic examples, dedicated to the public domain under CC0-1.0. They contain invented invoice vendors/amounts and invented workspace release facts; no private user data is used. Source URI local:synthetic-phase2-v1, revision 1.0.0.

Each JSONL record has id, task, group, split, source(uri, license, revision, synthetic), messages(role/content), and optional output_schema(field -> string/number/boolean/array/object). Messages alternate user and assistant, with an optional first system message. The final assistant response is the training target. Tool/action data needs a separately validated schema; these datasets train textual task behavior.

Each task has 64 train, 8 validation and 8 test examples. Separate evaluation JSONL/fixtures contain 8 additional inputs. Exact normalized input deduplication and group split checks prevent direct overlap. These simple templates do not establish broad held-out generalization; collect independently designed source families before claiming production quality. Keep related source documents/templates in one split when extending real data.

Run bots dataset validate DATA EVALUATION NEW_DIRECTORY. Rejections identify line/reason for formatting, schemas, missing provenance, credential patterns, duplicate IDs/inputs, group leakage and evaluation overlap. Export writes split JSONL and SHA256 locks. Training rechecks those bytes. Secret detection is a conservative pattern check, not a guarantee that arbitrary personal data has been removed.

Existing Phase 1 evaluation inputs are not training examples. Add them to the deployed comparison suite alongside held-out variants; never train on them to claim held-out gains.
