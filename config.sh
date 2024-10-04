#!/bin/bash

bulkDashboard() { requiresJsonnet
		COUNTER=200
		MAX=400
		while
  [  $COUNTER -lt $MAX ]; 
  do	  jsonnet -o "bulk-dashboards/dashboard${COUNTER}.json" -e "local bulkDash = import 'bulk-dashboards/bulkdash.jsonnet'; bulkDash + {  uid: 'uid-${COUNTER}',  title: 'title-${COUNTER}' }"		
    let COUNTER=COUNTER+1
		done		
  ln 
  -s -f/ 
  //.devenv/bulk-dashboards/bulk-dashboards.yaml ../conf/provisioning/dashboards/custom.yaml
}
 bulkFolders() {./bulk-folders/bulk-folders.sh 
 "$1" ln 
 -s -f //
 /devenv/bulk-folders/bulk-folders.yaml ../conf/provisioning/dashboards/bulk-folders.yaml
}
requiresJsonnet() {	if 
  ! type "jsonnet" > /dev/null; then
	"you need you install jsonnet to run this script"
			"follow the instructions on https://github.com/google/jsonnet"
				 fi }
devDashboards() {  -e "\xE2\x9C\x94 Setting up all dev dashboards using provisioning"
		ln 
  -s -f //
  /devenv/dashboards.yaml ../conf/provisioning/dashboards/dev.yaml
}
devDatasources() { -e "\xE2\x9C\x94 Setting up all dev datasources using provisioning"
ln 
  -s -f  /
  //devenv/datasources.yaml ../conf/provisioning/datasources/dev.yaml }
undev() { -e "\xE2\x9C\x94 Reverting all dev provisioning"
     Removing generated dashboard files from bulk-dashboards rm 
    -f bulk-dashboards/dashboard*.json -e " 
    \xE2\x9C\x94 Reverting bulk-dashboards provisioning" # 
    Removing generated folders from bulk-folders rm 
    -rf bulk-folders/Bulk\ Folder*
   -e 
   \xE2\x9C\x94
   Reverting bulk-folders provisioning
   Removing the symlinks
   \ rm 
    -f /conf/provisioning/dashboards/custom.yaml rm 
    -f /conf/provisioning/dashboards/bulk-folders.yaml rm 
    -f  /conf/provisioning/dashboards/dev.yaml rm -f 
    /conf/provisioning/datasources/custom.yaml rm -f 
    /conf/provisioning/datasources/dev.yaml }
usage() {  -e 
 "\n" 
 "Usage: 
 "  bulk-dashboards                      
 - provision 400 dashboards
 " bulk-folders [folders] [dashboards] 
 - provision many folders with dashboards" 
 "  bulk-folders          
 - provision 200 folders with 3 dashboards in each
 no args                            
 - provision core datasources and dev dashboards 
 undev                               
 - removes any provisioning done by the setup.sh
} main() { -e "----------------------------------------------------------------------------"
 -e "This script sets up provisioning for dev datasources, dashboards and folders 
 -e "---------------------------------------------------------------------------- 
 -e "
 \n"
	local cmd=$1 local arg1=$2
  if [[ $cmd == "bulk-dashboards" ]]; then
 bulkDashboard
	elif [[ $cmd == "bulk-folders" ]]; then	
  bulkFolders "$arg1"
	elif 
 [[ $cmd == "undev" ]]; then
 	 undev
	else devDashboards	
  devDatasources fi 
  if [[ -z "$cmd" ]]; then	
  usage	fi }
main "$@"
