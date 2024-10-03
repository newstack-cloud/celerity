use std::fmt;

use http::header::ToStrError;

/// Provides an error type for signature verification failures.
#[derive(Debug)]
pub enum SignatureVerificationError {
    /// An error indicating that the signature is invalid
    /// due to it having expired, being malformed, or not matching
    /// the expected signature.
    InvalidSignature(String),
    /// An error indicating that the signature header is missing
    /// from the request headers.
    SignatureHeadingMissing,
    /// An error indicating that the date header is missing
    /// from the request headers.
    DateHeaderMissing,
    /// An error indicating that one or more custom headers
    /// defined in the signature header are missing from the request headers.
    CustomHeadersMissing(Vec<String>),
}

impl fmt::Display for SignatureVerificationError {
    fn fmt(&self, f: &mut fmt::Formatter) -> fmt::Result {
        match self {
            SignatureVerificationError::InvalidSignature(msg) => {
                write!(
                    f,
                    "signature verification failed due to an invalid signature: {}",
                    msg,
                )
            }
            SignatureVerificationError::SignatureHeadingMissing => {
                write!(
                    f,
                    "signature verification failed due to the signature header\
                     not being in the provided request headers"
                )
            }
            SignatureVerificationError::DateHeaderMissing => {
                write!(
                    f,
                    "signature verification failed due to the date header\
                     not being in the provided request headers"
                )
            }
            SignatureVerificationError::CustomHeadersMissing(headers) => {
                write!(
                    f,
                    "signature verification failed due to the following custom headers\
                     not being in the provided request headers: {:?}",
                    headers,
                )
            }
        }
    }
}

impl From<ToStrError> for SignatureVerificationError {
    fn from(error: ToStrError) -> Self {
        SignatureVerificationError::InvalidSignature(error.to_string())
    }
}

impl From<SignatureMessageCreationError> for SignatureVerificationError {
    fn from(error: SignatureMessageCreationError) -> Self {
        match error {
            SignatureMessageCreationError::DateHeaderMissing => {
                SignatureVerificationError::DateHeaderMissing
            }
            SignatureMessageCreationError::CustomHeadersMissing(headers) => {
                SignatureVerificationError::CustomHeadersMissing(headers)
            }
            SignatureMessageCreationError::InvalidDateHeader => {
                SignatureVerificationError::InvalidSignature("Date header is invalid".to_string())
            }
            SignatureMessageCreationError::InvalidCustomHeader => {
                SignatureVerificationError::InvalidSignature("Custom header is invalid".to_string())
            }
            SignatureMessageCreationError::UnknownError(msg) => {
                SignatureVerificationError::InvalidSignature(msg)
            }
        }
    }
}

/// Provides an error type for failures when preparing a message
/// to be signed.
#[derive(Debug)]
pub enum SignatureMessageCreationError {
    /// An error indicating that the date header is invalid.
    InvalidDateHeader,
    /// An error indicating that the date header is missing
    /// from the request headers.
    DateHeaderMissing,
    /// An error indicating that a custom header is invalid.
    InvalidCustomHeader,
    /// An error indicating that one or more custom headers
    /// defined in the signature header are missing from the request headers.
    CustomHeadersMissing(Vec<String>),
    /// An error indicating that an unknown error occurred.
    /// This is primarily to allow for the conversion of verification
    /// errors to message creation errors.
    UnknownError(String),
}

impl fmt::Display for SignatureMessageCreationError {
    fn fmt(&self, f: &mut fmt::Formatter) -> fmt::Result {
        match self {
            SignatureMessageCreationError::DateHeaderMissing => {
                write!(
                    f,
                    "signature message creation failed due to the date header\
                     not being in the provided request headers"
                )
            }
            SignatureMessageCreationError::CustomHeadersMissing(headers) => {
                write!(
                    f,
                    "signature message creation failed due to the following custom headers\
                     not being in the provided request headers: {:?}",
                    headers,
                )
            }
            SignatureMessageCreationError::InvalidDateHeader => {
                write!(
                    f,
                    "signature message creation failed due to the date header\
                     being invalid"
                )
            }
            SignatureMessageCreationError::InvalidCustomHeader => {
                write!(
                    f,
                    "signature message creation failed due to a custom header\
                     being invalid"
                )
            }
            SignatureMessageCreationError::UnknownError(msg) => {
                write!(
                    f,
                    "signature message creation failed due to an unknown error: {}",
                    msg,
                )
            }
        }
    }
}

impl From<SignatureVerificationError> for SignatureMessageCreationError {
    fn from(error: SignatureVerificationError) -> Self {
        match error {
            SignatureVerificationError::DateHeaderMissing => {
                SignatureMessageCreationError::DateHeaderMissing
            }
            SignatureVerificationError::CustomHeadersMissing(headers) => {
                SignatureMessageCreationError::CustomHeadersMissing(headers)
            }
            SignatureVerificationError::InvalidSignature(msg) => {
                SignatureMessageCreationError::UnknownError(msg)
            }
            SignatureVerificationError::SignatureHeadingMissing => {
                SignatureMessageCreationError::DateHeaderMissing
            }
        }
    }
}
