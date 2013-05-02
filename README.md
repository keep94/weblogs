weblogs
=======

Easily add web access logs to your go http server.

## Using

	import "github.com/keep94/weblogs"

## Installing

	go get github.com/keep94/weblogs

## Features

If server panics before sending a response, weblogs automatically sends a
500 error to client and logs the panic.

## Online Documentation

Online documentation available [here](http://go.pkgdoc.org/github.com/keep94/weblogs).

## Dependencies

This package depends on [github.com/gorilla/context](http://github.com/gorilla/context).

## Example Usage

	handler := context.ClearHandler(weblogs.Handler(http.DefaultServeMux))
	http.ListenAndServe(":80", handler)
