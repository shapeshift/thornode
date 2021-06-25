package types

import "fmt"

var UnavailableBlock error = fmt.Errorf("block is not yet available")
var FailOutputMatchCriteria error = fmt.Errorf("fail to get output matching criteria")
