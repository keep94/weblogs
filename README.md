weblogs
=======

Easily add web access logs to your go http server.

## Using

	import "github.com/keep94/weblogs"

## Installing

	go get github.com/keep94/weblogs

## Online Documentation

Online documentation available [here](http://go.pkgdoc.org/github.com/keep94/weblogs).

## Dependencies

This package depends on [github.com/gorilla/context](http://github.com/gorilla/context).

## Example Usage

	handler := context.ClearHandler(weblogs.Handler(http.DefaultServeMux))
	http.ListenAndServe(":80", handler)
