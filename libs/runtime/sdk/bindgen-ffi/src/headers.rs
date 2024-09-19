pub struct HttpHeaders {}

pub unsafe fn http_headers_create() -> *mut HttpHeaders {
    let headers = Box::new(HttpHeaders {});
    Box::into_raw(headers)
}

pub unsafe fn http_headers_destroy(headers: *mut HttpHeaders) {
    if !headers.is_null() {
        drop(Box::from_raw(headers));
    };
}
