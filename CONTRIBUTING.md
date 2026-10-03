# Contributing

Run `make check` before opening a pull request. Include a minimal fixture for changes to symbol discovery, references, or entry points. Changes that may introduce false positives need explicit safety tests. Describe user-visible behavior and verification in your pull request.

Keep Go application logic independent of compiler and HTTP DTOs. Use small handwritten fakes for external boundaries. Do not send source to a live API from ordinary tests. Public documentation, source, comments, and CLI output are in English.
