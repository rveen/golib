# A Go API for Polarion

(This is a hack)

Polarion is an ALM tool (now) from Siemens (See https://polarion.plm.automation.siemens.com).
Polarion stores its data in Subversion repositories in XML format, with
one repository per project. The current API is a read-only view of a working copy
of just one revision of the data.
