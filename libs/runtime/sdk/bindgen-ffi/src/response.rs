use std::{collections::HashMap, ffi::CStr};

use axum::{body::Body, response::IntoResponse};

use crate::HttpHeaders;

pub struct Response {
    pub status: u16,
    pub headers: HttpHeaders,
    pub body: Option<String>,
    pub send_resp_channel: Option<tokio::sync::oneshot::Sender<ResponseData>>,
}

impl Response {
    pub fn send(&mut self) {
        if let Some(tx) = self.send_resp_channel.take() {
            let _ = tx.send(ResponseData {
                status: self.status,
                headers: None,
                body: self.body.take(),
            });
        }
    }
}

pub struct ResponseData {
    pub status: u16,
    pub headers: Option<HashMap<String, String>>,
    pub body: Option<String>,
}

impl IntoResponse for ResponseData {
    fn into_response(self) -> axum::response::Response<Body> {
        let mut builder = axum::response::Response::builder();
        for (key, value) in self.headers.unwrap_or_default() {
            builder = builder.header(key, value);
        }
        builder = builder.status(self.status);
        builder
            .body(Body::from(self.body.unwrap_or_default()))
            .unwrap()
    }
}

pub unsafe fn response_create(
    status: u16,
    headers: *mut HttpHeaders,
    body: &CStr,
) -> *mut Response {
    let response = Box::new(Response {
        status,
        headers: HttpHeaders {},
        body: Some(body.to_string_lossy().to_string()),
        send_resp_channel: None,
    });
    Box::into_raw(response)
}

pub unsafe fn response_set_status(response: *mut Response, status: u16) {
    if !response.is_null() {
        (*response).status = status;
    };
}

pub unsafe fn response_set_headers(response: *mut Response, headers: *mut HttpHeaders) {
    if !response.is_null() {
        (*response).headers = HttpHeaders {};
    };
}

pub unsafe fn response_send(response: *mut Response, body: &CStr) {
    if !response.is_null() {
        (*response).body = Some(body.to_string_lossy().to_string());
        (*response).send();
    };
}

pub unsafe fn response_destroy(response: *mut Response) {
    if !response.is_null() {
        drop(Box::from_raw(response));
    };
}
