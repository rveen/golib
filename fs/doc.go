// This package implements a file system 'browser' for use in a web server. By
// specifying paths in the file system, it returns directories, files and part of files
// (specific types of files) that correspond to that path. The particularity of this
// package is that it allows navigation into either conventional or versioned file
// systems (such as Subversion or Git), and into data files (only OGDL at the moment).
// Use of file extensions is optional (if the file name is unique).
//
// For now this is a read-only implementation. The content of a path is returned if found,
// but the file system cannot be modified.
//
// When the path points to a directory that contains an index.* file
// it returnes this file along with the directory list. Presence of several
// index.* files is not supported except in special cases: index.nolist(= do not return
// directory list).
//
// Paths
//
// Paths are a sequence of elements separated by slashes, following the Unix / Linux
// notation. Two special cases exist:
//
// * _n, where n is a number, is interpreted as a release number, and removed from
// the path. Instead a "revision" parameter is added to fe.Param().
//
// * If an element is not found in a directory but the directory contains a _token
// entry, that one is followed. A parameter is attached to fe.Params() with the
// token as name and the element as value.
//
// Example
//
// The two main functions of this package are New and Get.
//
//   fs := fs.New("/dir")
//   fe := fs.Get("file")
//
// Get returns a FileEntry object that implements the os.FileInfo interface and
// holds also the content (directoty list, file).
//
// TODO: for what os.FileInfo ??
//
// Templates
//
// File extensions that are configured as OGDL templates are preprocessed as such,
// that is, they are parsed and converted into an OGDL object accessible through fe.Tree().
// TODO: should this be done outside of this package?? (caching is a reason to do
// it here)
//
//
// Navigating data files
//
// Navigation within an OGDL file is handled over to the ogdl package (ogdl.Get).
//
// Navigating documents
//
// Markdown document navigation is handled by the document package (document.Get).
//
// Database navigation
//
// Not supported (done through templates).
//
// Relation between path and template
//
// Is this a fixed relation or can we specify a template for a path in an elegant
// way ? Or is it better to just write a template with the query or path inside ?
// Are we mixing functions?
//
// Revision list
//
// How to obtain the log of a path and use it in a template.
//
//   g := fs.Log(path)