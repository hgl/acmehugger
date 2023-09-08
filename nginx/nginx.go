package nginx

import "github.com/hgl/acmehugger"

var ConfDir = "/etc/nginx"
var Conf = ConfDir + "/nginx.conf"
var ConfOutDir = acmehugger.StateDir + "/nginx/conf"
