module trojanpanelnext/nodebundle

go 1.19

require (
	filippo.io/age v1.1.1
	golang.org/x/term v0.3.0
	gopkg.in/yaml.v3 v3.0.1
	trojanpanelnext/revocationreceipt v0.0.0
)

replace trojanpanelnext/revocationreceipt => ../revocationreceipt

require (
	golang.org/x/crypto v0.4.0 // indirect
	golang.org/x/sys v0.7.0 // indirect
)
