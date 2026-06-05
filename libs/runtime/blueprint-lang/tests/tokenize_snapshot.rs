//! Golden snapshot of the full token stream for a representative blueprint.
//!
//! Unlike the fine-grained unit tests in `lexer.rs`, this asserts the entire
//! token stream including comments, newlines, and positions for a single
//! mixed fixture, exercising the lexer end to end (declarations, strings,
//! single- and multi-line, `${..}` interpolation, operators, numbers).

use celerity_blueprint_lang::tokenize;

/// Renders the token stream of `src` to one line per token: the token kind, its
/// source value (debug-escaped so newlines stay on one line), and its
/// start/end position.
fn render(src: &str) -> String {
    let (tokens, errs) = tokenize(src);
    assert!(errs.is_none(), "unexpected lex errors: {errs:?}");
    tokens
        .iter()
        .map(|t| {
            format!(
                "{:?} {:?} @ {}:{}-{}:{}",
                t.ty, t.value, t.start.line, t.start.column, t.end.line, t.end.column
            )
        })
        .collect::<Vec<_>>()
        .join("\n")
}

#[test]
fn tokenize_mixed_blueprint() {
    let src = "version \"2025-11-02\"\n\
\n\
# A resource with interpolation and a multi-line description.\n\
resource saveOrder: aws/lambda/function {\n\
    description = \"\"\"\n\
        Saves an order to the store.\n\
        \"\"\"\n\
\n\
    spec {\n\
        memorySize = if(values.isProd, 1024, 512)\n\
        bucketName = \"${values.namePrefix}-orders\"\n\
    }\n\
}\n";
    insta::assert_snapshot!("mixed_blueprint_tokens", render(src));
}
