"""Smoke test to ensure the evals-engine package is importable and the CI pipeline works."""

import evals_engine


def test_version_is_defined() -> None:
    assert evals_engine.__version__ == "0.1.0"