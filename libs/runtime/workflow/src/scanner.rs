/// A scanner that can be used to parse a template function call.
pub struct Scanner {
    pos: usize,
    chars: Vec<char>,
    // A stack of positions in the sequence to take a snapshot
    // before attempting to parse a subsequence.
    // This allows the scanner to backtrack to the snapshot position
    // if the subsequence fails to parse.
    start_pos_stack: Vec<usize>,
}

pub enum ScannerError {
    Character(usize),
    EndOfInput,
}

pub enum ScannerAction<T> {
    // If the next iteration of the scanner returns None, return T
    // without advancing the scanner position.
    Request(T),

    // If the next iteration of the scanner returns None, return None
    // without advancing the scanner position.
    Require,

    // Immediately advance the scanner position and return T.
    Return(T),
}

impl Scanner {
    pub fn new(input: &str) -> Self {
        Scanner {
            pos: 0,
            chars: input.chars().collect(),
            start_pos_stack: Vec::new(),
        }
    }

    /// Get the current position of the scanner,
    /// this is useful for error reporting.
    pub fn pos(&self) -> usize {
        self.pos
    }

    /// Save the current position of the scanner
    /// as a way to backtrack from a failed parse attempt.
    pub fn save_pos(&mut self) {
        self.start_pos_stack.push(self.pos);
    }

    /// Pop the last saved position of the scanner.
    /// This is used to disregard a saved position
    /// after parsing of a specific subsequence has succeeded.
    pub fn pop_pos(&mut self) {
        self.start_pos_stack.pop();
    }

    /// Restore the last saved position of the scanner.
    /// This is used to backtrack from a failed parse attempt.
    pub fn backtrack(&mut self) {
        if let Some(pos) = self.start_pos_stack.pop() {
            self.pos = pos;
        }
    }

    /// Get the next character in the input without
    /// consuming it.
    /// This is also known as "lookahead".
    pub fn peek(&self) -> Option<&char> {
        self.chars.get(self.pos)
    }

    /// Check if the scanner has reached the end of the input.
    pub fn is_end(&self) -> bool {
        self.pos == self.chars.len()
    }

    /// Returns the next character in the input
    /// (if available) and advances the scanner position.
    pub fn pop(&mut self) -> Option<&char> {
        match self.chars.get(self.pos) {
            Some(ch) => {
                self.pos += 1;

                Some(ch)
            }
            None => None,
        }
    }

    /// Returns true if the `target` character is found at the current
    /// position in the input and advances the scanner position.
    /// Otherwise, returns false without advancing the scanner position.
    pub fn take(&mut self, target: &char) -> bool {
        match self.chars.get(self.pos) {
            Some(ch) => {
                if target == ch {
                    self.pos += 1;
                    true
                } else {
                    false
                }
            }
            None => false,
        }
    }

    /// Allows a parser to transform an input character into another type.
    /// This will invoke the `cb` function once. If the result is `None`, return it
    /// and advance the position. Otherwise return `None` and leave the position unchanged.
    pub fn transform<T>(&mut self, cb: impl FnOnce(&char) -> Option<T>) -> Option<T> {
        match self.chars.get(self.pos) {
            Some(input) => match cb(input) {
                Some(output) => {
                    self.pos += 1;
                    Some(output)
                }
                None => None,
            },
            None => None,
        }
    }

    pub fn scan<T>(
        &mut self,
        cb: impl Fn(&str) -> Option<ScannerAction<T>>,
    ) -> Result<Option<T>, ScannerError> {
        let mut sequence = String::new();
        let mut require = false;
        let mut request = None;

        loop {
            match self.chars.get(self.pos) {
                Some(target) => {
                    sequence.push(*target);

                    match cb(&sequence) {
                        Some(ScannerAction::Return(result)) => {
                            self.pos += 1;

                            break Ok(Some(result));
                        }
                        Some(ScannerAction::Request(result)) => {
                            self.pos += 1;
                            require = false;
                            request = Some(result);
                        }
                        Some(ScannerAction::Require) => {
                            self.pos += 1;
                            require = true;
                        }
                        None => {
                            if require {
                                break Err(ScannerError::Character(self.pos));
                            } else {
                                break Ok(request);
                            }
                        }
                    }
                }
                None => {
                    if require {
                        break Err(ScannerError::EndOfInput);
                    } else {
                        break Ok(request);
                    }
                }
            }
        }
    }
}
