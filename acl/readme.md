# Simple access control library

See the [API documentation](https://godoc.org/github.com/rveen/golib/acl).

## Configuration file format

There are two sections. The rules section has the format

    who resource operation allow/deny
    
The groups section creates groups of people, or groups of groups:

    group subgroup subgroup ...
    
Where subgroup can be a user.

    # A comment
    [rules]
    * * * -
    * /static *
    purchasing /dept/purchasing *

    [groups]
    purchasing john alice bob
    
All rules are checked, in the same order as written. 

Paths (resources) refer to one or several consecutive path elements, not 
parts of them. For example:

    * /static *
    
allows all people to access the URLs "/static" and "/static/*", but not "/static2".
There is no support for wildcards or partial path elements.