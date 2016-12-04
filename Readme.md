# A chained GeoCoder.

See https://github.com/OpenDriversLog/goodl for usage examples.

# License 
This work is licensed under a [![License](https://i.creativecommons.org/l/by-nc-sa/4.0/80x15.png) Creative Commons Attribution-NonCommercial-ShareAlike 4.0 International License](https://creativecommons.org/licenses/by-nc-sa/4.0/).
To view a copy of this license, visit http://creativecommons.org/licenses/by-nc-sa/4.0/ or send a letter to Creative Commons, PO Box 1866, Mountain View, CA 94042, USA.

This is a GeoCoder that works by chaining different geocoding providers, by your needs.

Step 1 : go run main.go

Step 2 : http://localhost:6091/reverse/a/b/abc/50.910950/13.323350

(http://localhost:6091/reverse/:userId/:key/:reqId/:lat/:lng) where userId and key are dummy for now, reqId will be returned

## How 2 add a new server to chain :

If its a odl-geocoder - setup server & go run main.go
Add a new entry to defChainServers.json & restartServer or call http://currentServer:6091/reparseChain

## Change port :
go run main.go -port=NEWPORT