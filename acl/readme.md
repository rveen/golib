# Simple access control library

## Configuration file format

There are two sections. The rules section has the format

    who resource operation allow/deny
    
The groups section creates groups of people, or groups of groups.

    # A comment
    [rules]
    * * * -
    * /static/* *
    purchasing /dept/purchasing *

    [groups]
    john purchasing
    alice purchasing
