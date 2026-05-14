package optimizer

const hoursPerMonth = 730.0

// EstimateMonthlySavings approximates monthly cost delta using flat rates.
// currentCPU and targetCPU are in millicores; currentMem and targetMem in MiB.
// costPerCPUHour is currency per core-hour; costPerGBHour is per GiB-hour.
func EstimateMonthlySavings(currentCPU, currentMem, targetCPU, targetMem, costPerCPUHour, costPerGBHour float64) float64 {
	deltaCores := (currentCPU - targetCPU) / 1000.0
	deltaGiB := (currentMem - targetMem) / 1024.0
	return hoursPerMonth * (deltaCores*costPerCPUHour + deltaGiB*costPerGBHour)
}
