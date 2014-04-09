weblogs
=======

Easily add web access logs to your go http server.

This API is now stable. Any future changes will be backward compatible with
existing code. However, any future function or data structure in "draft"
mode may change in incompatible ways. Such function or data structure will
be clearly marked as "draft" in the documentation.

## Using

	import "github.com/keep94/weblogs"

## Installing

	go get github.com/keep94/weblogs

## Features

If server panics before sending a response, weblogs automatically sends a
500 error to client and logs the panic.

## Online Documentation

Online documentation available [here](http://godoc.org/github.com/keep94/weblogs).

## Dependencies

This package depends on [github.com/gorilla/context](http://github.com/gorilla/context).

## Example Usage

	handler := context.ClearHandler(weblogs.Handler(http.DefaultServeMux))
	http.ListenAndServe(":80", handler)
