// Copyright (c) 2026 thorsphere.
// All Rights Reserved. Use is governed by the Functional Source License v1.1
// (FSL-1.1-ALv2) that can be found in the LICENSE file.
package lpcli

// ReadLine exports readLine for tests. Only compiled during go test.
func (p *Prompter) ReadLine() (string, error) {
	return p.readLine()
}
