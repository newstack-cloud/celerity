#[derive(Debug, Clone, Copy, PartialEq, Eq, Default)]
pub struct ParseOptions {
    pub substitutions: SubstitutionMode,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Default)]
pub enum SubstitutionMode {
    /// Parse `${..}` into structured `Substitution` nodes (AST).
    #[default]
    Structured,
    /// Validate each `${..}`, then keep it as raw `${..}` source text.
    RawText,
}
