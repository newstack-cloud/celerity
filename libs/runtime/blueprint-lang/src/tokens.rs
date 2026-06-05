//! Lexical tokens produced by the [`crate::lexer`].

use crate::source::Position;

/// The kind of a lexical [`Token`].
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash)]
pub enum TokenType {
    // Structural delimiters.
    LeftBracket,
    RightBracket,
    LeftParen,
    RightParen,
    LeftBrace,
    RightBrace,

    // Punctuation / separators.
    Colon,
    Assign,
    Comma,
    Period,

    // Comparison operators.
    Eq,
    Neq,
    Lt,
    Gt,
    Lte,
    Gte,

    // Logical operators.
    And,
    Or,
    Not,

    // Other operators.
    Slash,
    Star,

    // Literals.
    IntLiteral,
    FloatLiteral,
    BoolLiteral,
    NoneLiteral,
    StringStart,
    StringEnd,
    StringLiteral,
    MultilineStringLiteral,
    InterpolationStart,
    InterpolationEnd,

    // Structure / trivia.
    Newline,
    Comment,
    Identifier,

    // Reference-namespace keywords.
    KeywordVariables,
    KeywordValues,
    KeywordDatasources,
    KeywordResources,
    KeywordChildren,
    KeywordElem,
    KeywordI,

    // Declaration-header keywords.
    KeywordVariable,
    KeywordValue,
    KeywordData,
    KeywordResource,
    KeywordInclude,
    KeywordExport,
    KeywordMetadata,

    // Sub-construct / statement keywords.
    KeywordSpec,
    KeywordSelect,
    KeywordFilter,
    KeywordForeach,
    KeywordAs,
    KeywordBy,
    KeywordLabel,
    KeywordVersion,
    KeywordTransform,

    // Filter operator words.
    KeywordNot,
    KeywordIn,
    KeywordHas,
    KeywordKey,
    KeywordContains,
    KeywordStarts,
    KeywordWith,
    KeywordEnds,

    // Built-in type-name keywords.
    KeywordString,
    KeywordInteger,
    KeywordFloat,
    KeywordBoolean,
    KeywordArray,
    KeywordObject,

    Eof,
}

impl TokenType {
    /// Returns the reserved word for a keyword token, or `None` for any
    /// non-keyword token. This is the reverse of [`keyword_token`].
    pub fn keyword_word(self) -> Option<&'static str> {
        KEYWORDS
            .iter()
            .find_map(|(word, ty)| (*ty == self).then_some(*word))
    }

    /// Reports whether this token type is a reserved word.
    pub fn is_keyword(self) -> bool {
        self.keyword_word().is_some()
    }

    /// Returns a human-friendly label for use in diagnostics: the literal
    /// symbol for punctuation/operators, `keyword "x"` for reserved words, and
    /// a category name for value-bearing or structural tokens.
    pub fn display_label(self) -> String {
        if let Some(word) = self.keyword_word() {
            return format!("keyword {word:?}");
        }
        let label = match self {
            TokenType::LeftBracket => "'['",
            TokenType::RightBracket => "']'",
            TokenType::LeftParen => "'('",
            TokenType::RightParen => "')'",
            TokenType::LeftBrace => "'{'",
            TokenType::RightBrace => "'}'",
            TokenType::Colon => "':'",
            TokenType::Assign => "'='",
            TokenType::Comma => "','",
            TokenType::Period => "'.'",
            TokenType::Eq => "'=='",
            TokenType::Neq => "'!='",
            TokenType::Lt => "'<'",
            TokenType::Gt => "'>'",
            TokenType::Lte => "'<='",
            TokenType::Gte => "'>='",
            TokenType::And => "'&&'",
            TokenType::Or => "'||'",
            TokenType::Not => "'!'",
            TokenType::Slash => "'/'",
            TokenType::Star => "'*'",
            TokenType::IntLiteral => "integer",
            TokenType::FloatLiteral => "float",
            TokenType::BoolLiteral => "boolean",
            TokenType::NoneLiteral => "none",
            TokenType::StringStart => "string",
            TokenType::StringEnd => "end of string",
            TokenType::StringLiteral => "string",
            TokenType::MultilineStringLiteral => "multi-line string",
            TokenType::InterpolationStart => "'${'",
            TokenType::InterpolationEnd => "'}'",
            TokenType::Newline => "end of line",
            TokenType::Comment => "comment",
            TokenType::Identifier => "identifier",
            TokenType::Eof => "end of input",
            // Keyword variants are handled above by `keyword_word`.
            _ => "token",
        };
        label.to_string()
    }
}

/// The reserved-word table: maps each keyword to its [`TokenType`]. Ports the
/// Go `keywordTable`.
pub const KEYWORDS: &[(&str, TokenType)] = &[
    ("variable", TokenType::KeywordVariable),
    ("value", TokenType::KeywordValue),
    ("data", TokenType::KeywordData),
    ("resource", TokenType::KeywordResource),
    ("include", TokenType::KeywordInclude),
    ("export", TokenType::KeywordExport),
    ("metadata", TokenType::KeywordMetadata),
    ("spec", TokenType::KeywordSpec),
    ("select", TokenType::KeywordSelect),
    ("filter", TokenType::KeywordFilter),
    ("foreach", TokenType::KeywordForeach),
    ("as", TokenType::KeywordAs),
    ("by", TokenType::KeywordBy),
    ("label", TokenType::KeywordLabel),
    ("version", TokenType::KeywordVersion),
    ("transform", TokenType::KeywordTransform),
    ("not", TokenType::KeywordNot),
    ("in", TokenType::KeywordIn),
    ("has", TokenType::KeywordHas),
    ("key", TokenType::KeywordKey),
    ("contains", TokenType::KeywordContains),
    ("starts", TokenType::KeywordStarts),
    ("with", TokenType::KeywordWith),
    ("ends", TokenType::KeywordEnds),
    ("string", TokenType::KeywordString),
    ("integer", TokenType::KeywordInteger),
    ("float", TokenType::KeywordFloat),
    ("boolean", TokenType::KeywordBoolean),
    ("array", TokenType::KeywordArray),
    ("object", TokenType::KeywordObject),
    ("variables", TokenType::KeywordVariables),
    ("values", TokenType::KeywordValues),
    ("datasources", TokenType::KeywordDatasources),
    ("resources", TokenType::KeywordResources),
    ("children", TokenType::KeywordChildren),
    ("elem", TokenType::KeywordElem),
    ("i", TokenType::KeywordI),
];

/// Returns the keyword [`TokenType`] for `word`, or `None` if `word` is not a
/// reserved word (and is therefore an ordinary identifier).
pub fn keyword_token(word: &str) -> Option<TokenType> {
    KEYWORDS
        .iter()
        .find_map(|(kw, ty)| (*kw == word).then_some(*ty))
}

/// A single lexical token carrying its source position range. `start` is
/// inclusive and `end` is exclusive (the position immediately after the
/// token's final character).
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Token {
    pub ty: TokenType,
    pub value: String,
    pub start: Position,
    pub end: Position,
}

impl Token {
    /// Creates a token of the given type spanning `start..end` with the given
    /// source text value.
    pub fn new(ty: TokenType, value: impl Into<String>, start: Position, end: Position) -> Self {
        Self {
            ty,
            value: value.into(),
            start,
            end,
        }
    }
}
