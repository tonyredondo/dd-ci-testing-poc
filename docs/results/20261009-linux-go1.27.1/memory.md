# Aggregate memory comparisons

Each cell is the median of the per-run `memory.peak` values, in MiB
(1 MiB = 1,048,576 bytes). The cgroup includes all build or test processes,
detached daemons, nested builds and the measurement helper. Charged file-cache
pages and kernel memory are included. This is neither a Go heap measurement
nor the sum of independently observed process RSS peaks. Runtime receivers run
outside the measured cgroup. Warmups and build qualification commands are excluded.

The percentages compare the same memory metric against Orchestrion first and
Native second. Failed variants have no comparative median; their individual
peaks remain in the raw records.

## Cold compilation

| Project | Flags | CPUs | Native | Orchestrion | POC Mini |
| --- | --- | ---: | ---: | ---: | ---: |
| Gin | `none` | 4 | 1674.3 MiB | 2480.6 MiB | 1759.7 MiB (-29.1%; +5.1%) |
| Gin | `none` | 32 | 1851.3 MiB | 2639.9 MiB | 1858.7 MiB (-29.6%; +0.4%) |
| Chi | `none` | 4 | 464.5 MiB | 1292.0 MiB | 533.3 MiB (-58.7%; +14.8%) |
| Chi | `none` | 32 | 699.1 MiB | 2021.4 MiB | 755.6 MiB (-62.6%; +8.1%) |
| Gin | `-race` | 4 | 1694.1 MiB | 2697.3 MiB | 1747.5 MiB (-35.2%; +3.2%) |
| Gin | `-race` | 32 | 1738.9 MiB | 2693.4 MiB | 1837.4 MiB (-31.8%; +5.7%) |
| Gin | `-cover` | 4 | 1685.3 MiB | 2624.1 MiB | 1851.6 MiB (-29.4%; +9.9%) |
| Gin | `-cover` | 32 | 1850.4 MiB | 2862.9 MiB | 1906.9 MiB (-33.4%; +3.0%) |
| Gin | `-coverpkg=./... -covermode=atomic` | 4 | 1685.4 MiB | 2598.8 MiB | 1785.0 MiB (-31.3%; +5.9%) |
| Gin | `-coverpkg=./... -covermode=atomic` | 32 | 1890.4 MiB | 2713.6 MiB | 1926.4 MiB (-29.0%; +1.9%) |
| Gin | `-race -coverpkg=./... -covermode=atomic` | 4 | 1756.8 MiB | 2792.1 MiB | 1810.0 MiB (-35.2%; +3.0%) |
| Gin | `-race -coverpkg=./... -covermode=atomic` | 32 | 1790.8 MiB | 2726.9 MiB | 1868.6 MiB (-31.5%; +4.3%) |
| Chi | `-race` | 4 | 415.8 MiB | 1390.6 MiB | 572.1 MiB (-58.9%; +37.6%) |
| Chi | `-race` | 32 | 665.4 MiB | 1674.4 MiB | 691.7 MiB (-58.7%; +4.0%) |
| Chi | `-cover` | 4 | 466.0 MiB | 1308.2 MiB | 537.1 MiB (-58.9%; +15.3%) |
| Chi | `-cover` | 32 | 746.3 MiB | 2152.3 MiB | 801.7 MiB (-62.8%; +7.4%) |
| Chi | `-coverpkg=./... -covermode=atomic` | 4 | 479.6 MiB | 1306.7 MiB | 532.6 MiB (-59.2%; +11.0%) |
| Chi | `-coverpkg=./... -covermode=atomic` | 32 | 762.6 MiB | 2137.0 MiB | 814.5 MiB (-61.9%; +6.8%) |
| Chi | `-race -coverpkg=./... -covermode=atomic` | 4 | 431.9 MiB | 1395.4 MiB | 583.6 MiB (-58.2%; +35.1%) |
| Chi | `-race -coverpkg=./... -covermode=atomic` | 32 | 642.9 MiB | 1608.4 MiB | 688.3 MiB (-57.2%; +7.1%) |
| Testify Direct | `none` | 4 | 446.0 MiB | 1015.8 MiB | 524.4 MiB (-48.4%; +17.6%) |
| Testify Direct | `none` | 32 | 732.8 MiB | 1319.7 MiB | 803.0 MiB (-39.2%; +9.6%) |
| Testify Direct | `-race` | 4 | 400.5 MiB | 1029.6 MiB | 538.1 MiB (-47.7%; +34.3%) |
| Testify Direct | `-race` | 32 | 594.0 MiB | 1182.8 MiB | 723.1 MiB (-38.9%; +21.7%) |
| Testify Direct | `-cover` | 4 | 409.4 MiB | 992.5 MiB | 535.2 MiB (-46.1%; +30.7%) |
| Testify Direct | `-cover` | 32 | 697.3 MiB | 1444.5 MiB | 745.5 MiB (-48.4%; +6.9%) |
| Testify Direct | `-coverpkg=./... -covermode=atomic` | 4 | 404.1 MiB | 992.9 MiB | 526.5 MiB (-47.0%; +30.3%) |
| Testify Direct | `-coverpkg=./... -covermode=atomic` | 32 | 662.7 MiB | 1405.3 MiB | 730.8 MiB (-48.0%; +10.3%) |
| Testify Direct | `-race -coverpkg=./... -covermode=atomic` | 4 | 395.1 MiB | 968.7 MiB | 581.6 MiB (-40.0%; +47.2%) |
| Testify Direct | `-race -coverpkg=./... -covermode=atomic` | 32 | 591.0 MiB | 1196.5 MiB | 633.1 MiB (-47.1%; +7.1%) |
| Testify Direct | `-coverpkg=testing,github.com/stretchr/testify/suite -covermode=atomic` | 4 | 436.9 MiB | 1012.0 MiB | 532.1 MiB (-47.4%; +21.8%) |
| Testify Direct | `-coverpkg=testing,github.com/stretchr/testify/suite -covermode=atomic` | 32 | 728.2 MiB | 1374.6 MiB | 709.0 MiB (-48.4%; -2.6%) |
| Testify Direct | `-race -coverpkg=testing,github.com/stretchr/testify/suite -covermode=atomic` | 4 | 412.4 MiB | 1093.2 MiB | 565.2 MiB (-48.3%; +37.1%) |
| Testify Direct | `-race -coverpkg=testing,github.com/stretchr/testify/suite -covermode=atomic` | 32 | 638.6 MiB | 1247.0 MiB | 627.7 MiB (-49.7%; -1.7%) |
| Testify External | `none` | 4 | 436.4 MiB | 1007.9 MiB | 524.7 MiB (-47.9%; +20.2%) |
| Testify External | `none` | 32 | 711.6 MiB | 1358.1 MiB | 769.6 MiB (-43.3%; +8.2%) |
| Testify External | `-race` | 4 | 395.3 MiB | 1080.2 MiB | 564.2 MiB (-47.8%; +42.7%) |
| Testify External | `-race` | 32 | 627.3 MiB | 1207.5 MiB | 645.8 MiB (-46.5%; +2.9%) |
| Testify External | `-cover` | 4 | 420.4 MiB | 1014.6 MiB | 532.2 MiB (-47.5%; +26.6%) |
| Testify External | `-cover` | 32 | 748.0 MiB | 1348.5 MiB | 810.4 MiB (-39.9%; +8.3%) |
| Testify External | `-coverpkg=./... -covermode=atomic` | 4 | 420.3 MiB | 1014.8 MiB | 532.7 MiB (-47.5%; +26.7%) |
| Testify External | `-coverpkg=./... -covermode=atomic` | 32 | 692.0 MiB | 1306.1 MiB | 798.2 MiB (-38.9%; +15.3%) |
| Testify External | `-race -coverpkg=./... -covermode=atomic` | 4 | 408.0 MiB | 1071.2 MiB | 580.4 MiB (-45.8%; +42.3%) |
| Testify External | `-race -coverpkg=./... -covermode=atomic` | 32 | 612.2 MiB | 1206.7 MiB | 727.9 MiB (-39.7%; +18.9%) |
| Testify External | `-coverpkg=testing,github.com/stretchr/testify/suite -covermode=atomic` | 4 | 455.9 MiB | 1025.8 MiB | 538.2 MiB (-47.5%; +18.1%) |
| Testify External | `-coverpkg=testing,github.com/stretchr/testify/suite -covermode=atomic` | 32 | 694.1 MiB | 1293.6 MiB | 804.8 MiB (-37.8%; +16.0%) |
| Testify External | `-race -coverpkg=testing,github.com/stretchr/testify/suite -covermode=atomic` | 4 | 414.6 MiB | 1089.6 MiB | 590.8 MiB (-45.8%; +42.5%) |
| Testify External | `-race -coverpkg=testing,github.com/stretchr/testify/suite -covermode=atomic` | 32 | 617.8 MiB | 1241.4 MiB | 724.2 MiB (-41.7%; +17.2%) |

## Cached compilation — unchanged output reused

| Project | Flags | CPUs | Native | Orchestrion | POC Mini |
| --- | --- | ---: | ---: | ---: | ---: |
| Gin | `none` | 4 | 49.9 MiB | 85.4 MiB | 60.8 MiB (-28.8%; +21.9%) |
| Gin | `none` | 32 | 67.0 MiB | 117.3 MiB | 81.3 MiB (-30.7%; +21.4%) |
| Chi | `none` | 4 | 41.8 MiB | 73.6 MiB | 52.8 MiB (-28.2%; +26.2%) |
| Chi | `none` | 32 | 57.0 MiB | 108.3 MiB | 73.1 MiB (-32.5%; +28.3%) |
| Gin | `-race` | 4 | 50.3 MiB | 86.4 MiB | 61.3 MiB (-29.1%; +21.7%) |
| Gin | `-race` | 32 | 68.8 MiB | 119.4 MiB | 83.8 MiB (-29.8%; +21.7%) |
| Gin | `-cover` | 4 | 66.9 MiB | 99.0 MiB | 77.9 MiB (-21.3%; +16.4%) |
| Gin | `-cover` | 32 | 87.1 MiB | 148.8 MiB | 104.7 MiB (-29.7%; +20.1%) |
| Gin | `-coverpkg=./... -covermode=atomic` | 4 | 65.6 MiB | 109.4 MiB | 77.0 MiB (-29.6%; +17.4%) |
| Gin | `-coverpkg=./... -covermode=atomic` | 32 | 90.0 MiB | 139.0 MiB | 105.6 MiB (-24.0%; +17.4%) |
| Gin | `-race -coverpkg=./... -covermode=atomic` | 4 | 68.5 MiB | 100.4 MiB | 64.7 MiB (-35.6%; -5.6%) |
| Gin | `-race -coverpkg=./... -covermode=atomic` | 32 | 91.2 MiB | 142.7 MiB | 105.5 MiB (-26.1%; +15.6%) |
| Chi | `-race` | 4 | 41.9 MiB | 79.0 MiB | 55.0 MiB (-30.4%; +31.2%) |
| Chi | `-race` | 32 | 60.1 MiB | 109.4 MiB | 73.0 MiB (-33.3%; +21.5%) |
| Chi | `-cover` | 4 | 42.0 MiB | 77.0 MiB | 51.3 MiB (-33.4%; +22.2%) |
| Chi | `-cover` | 32 | 62.5 MiB | 118.0 MiB | 74.8 MiB (-36.6%; +19.8%) |
| Chi | `-coverpkg=./... -covermode=atomic` | 4 | 43.7 MiB | 79.8 MiB | 55.1 MiB (-31.0%; +25.9%) |
| Chi | `-coverpkg=./... -covermode=atomic` | 32 | 58.7 MiB | 117.1 MiB | 73.6 MiB (-37.2%; +25.4%) |
| Chi | `-race -coverpkg=./... -covermode=atomic` | 4 | 43.4 MiB | 72.6 MiB | 55.4 MiB (-23.6%; +27.7%) |
| Chi | `-race -coverpkg=./... -covermode=atomic` | 32 | 58.8 MiB | 114.4 MiB | 72.6 MiB (-36.6%; +23.3%) |
| Testify Direct | `none` | 4 | 41.5 MiB | 72.3 MiB | 54.0 MiB (-25.3%; +30.1%) |
| Testify Direct | `none` | 32 | 58.0 MiB | 104.4 MiB | 75.9 MiB (-27.3%; +30.9%) |
| Testify Direct | `-race` | 4 | 41.8 MiB | 73.3 MiB | 52.6 MiB (-28.2%; +25.8%) |
| Testify Direct | `-race` | 32 | 46.2 MiB | 75.9 MiB | 78.6 MiB (+3.6%; +70.3%) |
| Testify Direct | `-cover` | 4 | 42.6 MiB | 68.1 MiB | 55.4 MiB (-18.6%; +30.3%) |
| Testify Direct | `-cover` | 32 | 43.6 MiB | 74.6 MiB | 74.1 MiB (-0.7%; +69.9%) |
| Testify Direct | `-coverpkg=./... -covermode=atomic` | 4 | 33.8 MiB | 59.4 MiB | 55.9 MiB (-5.9%; +65.4%) |
| Testify Direct | `-coverpkg=./... -covermode=atomic` | 32 | 43.2 MiB | 73.6 MiB | 73.1 MiB (-0.7%; +69.0%) |
| Testify Direct | `-race -coverpkg=./... -covermode=atomic` | 4 | 35.4 MiB | 56.8 MiB | 57.0 MiB (+0.5%; +61.2%) |
| Testify Direct | `-race -coverpkg=./... -covermode=atomic` | 32 | 57.5 MiB | 103.5 MiB | 74.0 MiB (-28.5%; +28.6%) |
| Testify Direct | `-coverpkg=testing,github.com/stretchr/testify/suite -covermode=atomic` | 4 | 41.5 MiB | 77.2 MiB | 54.3 MiB (-29.7%; +30.6%) |
| Testify Direct | `-coverpkg=testing,github.com/stretchr/testify/suite -covermode=atomic` | 32 | 57.2 MiB | 106.0 MiB | 72.2 MiB (-31.9%; +26.2%) |
| Testify Direct | `-race -coverpkg=testing,github.com/stretchr/testify/suite -covermode=atomic` | 4 | 41.4 MiB | 78.4 MiB | 56.8 MiB (-27.6%; +37.3%) |
| Testify Direct | `-race -coverpkg=testing,github.com/stretchr/testify/suite -covermode=atomic` | 32 | 57.9 MiB | 104.6 MiB | 60.9 MiB (-41.8%; +5.3%) |
| Testify External | `none` | 4 | 41.8 MiB | 72.4 MiB | 52.7 MiB (-27.2%; +26.0%) |
| Testify External | `none` | 32 | 54.3 MiB | 102.7 MiB | 69.1 MiB (-32.7%; +27.4%) |
| Testify External | `-race` | 4 | 42.4 MiB | 71.8 MiB | 53.8 MiB (-25.0%; +27.0%) |
| Testify External | `-race` | 32 | 58.1 MiB | 107.8 MiB | 75.3 MiB (-30.1%; +29.7%) |
| Testify External | `-cover` | 4 | 43.4 MiB | 72.4 MiB | 53.8 MiB (-25.8%; +24.0%) |
| Testify External | `-cover` | 32 | 57.3 MiB | 110.1 MiB | 73.5 MiB (-33.2%; +28.2%) |
| Testify External | `-coverpkg=./... -covermode=atomic` | 4 | 42.0 MiB | 74.4 MiB | 53.6 MiB (-28.0%; +27.5%) |
| Testify External | `-coverpkg=./... -covermode=atomic` | 32 | 58.8 MiB | 94.2 MiB | 73.3 MiB (-22.2%; +24.6%) |
| Testify External | `-race -coverpkg=./... -covermode=atomic` | 4 | 41.5 MiB | 70.1 MiB | 53.2 MiB (-24.2%; +28.1%) |
| Testify External | `-race -coverpkg=./... -covermode=atomic` | 32 | 56.6 MiB | 102.4 MiB | 73.0 MiB (-28.7%; +29.1%) |
| Testify External | `-coverpkg=testing,github.com/stretchr/testify/suite -covermode=atomic` | 4 | 42.7 MiB | 74.4 MiB | 54.5 MiB (-26.8%; +27.7%) |
| Testify External | `-coverpkg=testing,github.com/stretchr/testify/suite -covermode=atomic` | 32 | 57.9 MiB | 108.6 MiB | 78.1 MiB (-28.1%; +34.7%) |
| Testify External | `-race -coverpkg=testing,github.com/stretchr/testify/suite -covermode=atomic` | 4 | 42.0 MiB | 77.3 MiB | 57.3 MiB (-25.9%; +36.4%) |
| Testify External | `-race -coverpkg=testing,github.com/stretchr/testify/suite -covermode=atomic` | 32 | 60.0 MiB | 105.9 MiB | 76.3 MiB (-27.9%; +27.1%) |

## Warm dependencies — forced fresh link

| Project | Flags | CPUs | Native | Orchestrion | POC Mini |
| --- | --- | ---: | ---: | ---: | ---: |
| Gin | `none` | 4 | 340.3 MiB | 816.5 MiB | 448.5 MiB (-45.1%; +31.8%) |
| Gin | `none` | 32 | 411.5 MiB | 1141.9 MiB | 527.4 MiB (-53.8%; +28.2%) |
| Chi | `none` | 4 | 137.0 MiB | 377.7 MiB | 172.5 MiB (-54.3%; +25.9%) |
| Chi | `none` | 32 | 182.8 MiB | 429.9 MiB | 202.0 MiB (-53.0%; +10.5%) |
| Gin | `-race` | 4 | 429.4 MiB | 928.1 MiB | 530.6 MiB (-42.8%; +23.6%) |
| Gin | `-race` | 32 | 512.1 MiB | 1479.3 MiB | 605.0 MiB (-59.1%; +18.1%) |
| Gin | `-cover` | 4 | 351.6 MiB | 787.4 MiB | 454.4 MiB (-42.3%; +29.2%) |
| Gin | `-cover` | 32 | 405.2 MiB | 1147.3 MiB | 540.8 MiB (-52.9%; +33.5%) |
| Gin | `-coverpkg=./... -covermode=atomic` | 4 | 352.6 MiB | 797.0 MiB | 464.2 MiB (-41.8%; +31.7%) |
| Gin | `-coverpkg=./... -covermode=atomic` | 32 | 411.7 MiB | 1158.4 MiB | 531.7 MiB (-54.1%; +29.1%) |
| Gin | `-race -coverpkg=./... -covermode=atomic` | 4 | 436.7 MiB | 1000.6 MiB | 521.6 MiB (-47.9%; +19.4%) |
| Gin | `-race -coverpkg=./... -covermode=atomic` | 32 | 505.7 MiB | 1390.4 MiB | 644.4 MiB (-53.7%; +27.4%) |
| Chi | `-race` | 4 | 150.7 MiB | 434.1 MiB | 188.1 MiB (-56.7%; +24.8%) |
| Chi | `-race` | 32 | 178.9 MiB | 461.2 MiB | 219.1 MiB (-52.5%; +22.5%) |
| Chi | `-cover` | 4 | 140.1 MiB | 375.4 MiB | 175.1 MiB (-53.4%; +25.0%) |
| Chi | `-cover` | 32 | 182.2 MiB | 429.8 MiB | 210.6 MiB (-51.0%; +15.6%) |
| Chi | `-coverpkg=./... -covermode=atomic` | 4 | 141.1 MiB | 376.5 MiB | 180.2 MiB (-52.1%; +27.7%) |
| Chi | `-coverpkg=./... -covermode=atomic` | 32 | 184.0 MiB | 441.0 MiB | 205.4 MiB (-53.4%; +11.6%) |
| Chi | `-race -coverpkg=./... -covermode=atomic` | 4 | 151.1 MiB | 415.6 MiB | 194.2 MiB (-53.3%; +28.5%) |
| Chi | `-race -coverpkg=./... -covermode=atomic` | 32 | 184.3 MiB | 456.5 MiB | 230.3 MiB (-49.6%; +25.0%) |
| Testify Direct | `none` | 4 | 86.3 MiB | 242.6 MiB | 111.1 MiB (-54.2%; +28.7%) |
| Testify Direct | `none` | 32 | 116.8 MiB | 266.2 MiB | 140.1 MiB (-47.4%; +19.9%) |
| Testify Direct | `-race` | 4 | 95.1 MiB | 269.0 MiB | 122.0 MiB (-54.7%; +28.3%) |
| Testify Direct | `-race` | 32 | 97.7 MiB | 265.3 MiB | 147.2 MiB (-44.5%; +50.7%) |
| Testify Direct | `-cover` | 4 | 84.6 MiB | 229.6 MiB | 117.6 MiB (-48.8%; +39.0%) |
| Testify Direct | `-cover` | 32 | 93.9 MiB | 235.9 MiB | 141.2 MiB (-40.1%; +50.4%) |
| Testify Direct | `-coverpkg=./... -covermode=atomic` | 4 | 84.9 MiB | 222.9 MiB | 118.9 MiB (-46.7%; +40.0%) |
| Testify Direct | `-coverpkg=./... -covermode=atomic` | 32 | 91.3 MiB | 234.6 MiB | 142.0 MiB (-39.5%; +55.5%) |
| Testify Direct | `-race -coverpkg=./... -covermode=atomic` | 4 | 89.5 MiB | 253.9 MiB | 125.9 MiB (-50.4%; +40.6%) |
| Testify Direct | `-race -coverpkg=./... -covermode=atomic` | 32 | 116.8 MiB | 294.0 MiB | 143.9 MiB (-51.1%; +23.2%) |
| Testify Direct | `-coverpkg=testing,github.com/stretchr/testify/suite -covermode=atomic` | 4 | 88.5 MiB | 241.9 MiB | 114.6 MiB (-52.6%; +29.5%) |
| Testify Direct | `-coverpkg=testing,github.com/stretchr/testify/suite -covermode=atomic` | 32 | 122.2 MiB | 261.9 MiB | 131.0 MiB (-50.0%; +7.2%) |
| Testify Direct | `-race -coverpkg=testing,github.com/stretchr/testify/suite -covermode=atomic` | 4 | 95.1 MiB | 263.1 MiB | 128.9 MiB (-51.0%; +35.6%) |
| Testify Direct | `-race -coverpkg=testing,github.com/stretchr/testify/suite -covermode=atomic` | 32 | 118.7 MiB | 294.0 MiB | 130.2 MiB (-55.7%; +9.7%) |
| Testify External | `none` | 4 | 86.0 MiB | 236.9 MiB | 113.0 MiB (-52.3%; +31.3%) |
| Testify External | `none` | 32 | 116.0 MiB | 264.0 MiB | 121.5 MiB (-54.0%; +4.8%) |
| Testify External | `-race` | 4 | 93.8 MiB | 265.0 MiB | 126.4 MiB (-52.3%; +34.7%) |
| Testify External | `-race` | 32 | 117.4 MiB | 294.0 MiB | 148.7 MiB (-49.4%; +26.6%) |
| Testify External | `-cover` | 4 | 87.4 MiB | 244.0 MiB | 118.8 MiB (-51.3%; +35.9%) |
| Testify External | `-cover` | 32 | 119.0 MiB | 265.1 MiB | 140.5 MiB (-47.0%; +18.0%) |
| Testify External | `-coverpkg=./... -covermode=atomic` | 4 | 87.4 MiB | 242.2 MiB | 119.0 MiB (-50.9%; +36.1%) |
| Testify External | `-coverpkg=./... -covermode=atomic` | 32 | 115.3 MiB | 258.6 MiB | 137.9 MiB (-46.7%; +19.6%) |
| Testify External | `-race -coverpkg=./... -covermode=atomic` | 4 | 92.5 MiB | 265.1 MiB | 122.9 MiB (-53.6%; +32.9%) |
| Testify External | `-race -coverpkg=./... -covermode=atomic` | 32 | 118.8 MiB | 290.6 MiB | 147.7 MiB (-49.2%; +24.3%) |
| Testify External | `-coverpkg=testing,github.com/stretchr/testify/suite -covermode=atomic` | 4 | 88.2 MiB | 241.6 MiB | 114.9 MiB (-52.4%; +30.2%) |
| Testify External | `-coverpkg=testing,github.com/stretchr/testify/suite -covermode=atomic` | 32 | 118.5 MiB | 266.3 MiB | 140.7 MiB (-47.2%; +18.7%) |
| Testify External | `-race -coverpkg=testing,github.com/stretchr/testify/suite -covermode=atomic` | 4 | 93.3 MiB | 270.2 MiB | 123.6 MiB (-54.2%; +32.5%) |
| Testify External | `-race -coverpkg=testing,github.com/stretchr/testify/suite -covermode=atomic` | 32 | 115.7 MiB | 293.2 MiB | 148.7 MiB (-49.3%; +28.5%) |

## Incremental compilation — reachable test-body edit

| Project | Flags | CPUs | Native | Orchestrion | POC Mini |
| --- | --- | ---: | ---: | ---: | ---: |
| Gin | `none` | 4 | 230.3 MiB | 347.5 MiB | 240.9 MiB (-30.7%; +4.6%) |
| Gin | `none` | 32 | 282.6 MiB | 366.2 MiB | 296.7 MiB (-19.0%; +5.0%) |
| Chi | `none` | 4 | 130.8 MiB | 234.5 MiB | 140.4 MiB (-40.1%; +7.4%) |
| Chi | `none` | 32 | 180.2 MiB | 266.9 MiB | 190.0 MiB (-28.8%; +5.4%) |
| Gin | `-race` | 4 | 228.7 MiB | 384.3 MiB | 237.7 MiB (-38.2%; +3.9%) |
| Gin | `-race` | 32 | 243.3 MiB | 403.4 MiB | 258.7 MiB (-35.9%; +6.4%) |
| Gin | `-cover` | 4 | 244.2 MiB | 375.7 MiB | 256.1 MiB (-31.8%; +4.9%) |
| Gin | `-cover` | 32 | 317.7 MiB | 466.9 MiB | 334.3 MiB (-28.4%; +5.2%) |
| Gin | `-coverpkg=./... -covermode=atomic` | 4 | 254.0 MiB | 399.8 MiB | 274.6 MiB (-31.3%; +8.1%) |
| Gin | `-coverpkg=./... -covermode=atomic` | 32 | 323.7 MiB | 479.7 MiB | 342.4 MiB (-28.6%; +5.8%) |
| Gin | `-race -coverpkg=./... -covermode=atomic` | 4 | 240.8 MiB | 394.1 MiB | 259.9 MiB (-34.1%; +7.9%) |
| Gin | `-race -coverpkg=./... -covermode=atomic` | 32 | 262.1 MiB | 413.7 MiB | 279.8 MiB (-32.4%; +6.7%) |
| Chi | `-race` | 4 | 102.7 MiB | 254.8 MiB | 123.8 MiB (-51.4%; +20.5%) |
| Chi | `-race` | 32 | 129.1 MiB | 292.0 MiB | 150.6 MiB (-48.4%; +16.6%) |
| Chi | `-cover` | 4 | 137.5 MiB | 234.7 MiB | 152.6 MiB (-35.0%; +11.0%) |
| Chi | `-cover` | 32 | 174.1 MiB | 271.3 MiB | 189.8 MiB (-30.1%; +9.0%) |
| Chi | `-coverpkg=./... -covermode=atomic` | 4 | 140.1 MiB | 235.6 MiB | 154.7 MiB (-34.4%; +10.4%) |
| Chi | `-coverpkg=./... -covermode=atomic` | 32 | 174.9 MiB | 270.1 MiB | 190.9 MiB (-29.3%; +9.1%) |
| Chi | `-race -coverpkg=./... -covermode=atomic` | 4 | 110.9 MiB | 256.6 MiB | 127.8 MiB (-50.2%; +15.3%) |
| Chi | `-race -coverpkg=./... -covermode=atomic` | 32 | 137.9 MiB | 295.3 MiB | 148.5 MiB (-49.7%; +7.7%) |
| Testify Direct | `none` | 4 | 88.6 MiB | 241.8 MiB | 117.1 MiB (-51.6%; +32.2%) |
| Testify Direct | `none` | 32 | 133.5 MiB | 262.4 MiB | 154.1 MiB (-41.3%; +15.4%) |
| Testify Direct | `-race` | 4 | 91.0 MiB | 254.4 MiB | 123.1 MiB (-51.6%; +35.4%) |
| Testify Direct | `-race` | 32 | 100.3 MiB | 266.4 MiB | 149.6 MiB (-43.8%; +49.1%) |
| Testify Direct | `-cover` | 4 | 89.2 MiB | 231.8 MiB | 120.1 MiB (-48.2%; +34.6%) |
| Testify Direct | `-cover` | 32 | 95.2 MiB | 239.4 MiB | 145.1 MiB (-39.4%; +52.4%) |
| Testify Direct | `-coverpkg=./... -covermode=atomic` | 4 | 81.1 MiB | 220.3 MiB | 120.2 MiB (-45.4%; +48.2%) |
| Testify Direct | `-coverpkg=./... -covermode=atomic` | 32 | 95.5 MiB | 237.3 MiB | 144.6 MiB (-39.1%; +51.5%) |
| Testify Direct | `-race -coverpkg=./... -covermode=atomic` | 4 | 87.9 MiB | 255.9 MiB | 123.7 MiB (-51.6%; +40.8%) |
| Testify Direct | `-race -coverpkg=./... -covermode=atomic` | 32 | 121.1 MiB | 293.7 MiB | 146.5 MiB (-50.1%; +20.9%) |
| Testify Direct | `-coverpkg=testing,github.com/stretchr/testify/suite -covermode=atomic` | 4 | 89.5 MiB | 244.8 MiB | 118.1 MiB (-51.8%; +32.0%) |
| Testify Direct | `-coverpkg=testing,github.com/stretchr/testify/suite -covermode=atomic` | 32 | 132.9 MiB | 260.3 MiB | 131.2 MiB (-49.6%; -1.3%) |
| Testify Direct | `-race -coverpkg=testing,github.com/stretchr/testify/suite -covermode=atomic` | 4 | 99.7 MiB | 271.5 MiB | 125.7 MiB (-53.7%; +26.1%) |
| Testify Direct | `-race -coverpkg=testing,github.com/stretchr/testify/suite -covermode=atomic` | 32 | 118.1 MiB | 294.6 MiB | 135.9 MiB (-53.9%; +15.0%) |
| Testify External | `none` | 4 | 87.9 MiB | 241.9 MiB | 113.9 MiB (-52.9%; +29.5%) |
| Testify External | `none` | 32 | 133.3 MiB | 255.6 MiB | 123.2 MiB (-51.8%; -7.6%) |
| Testify External | `-race` | 4 | 94.6 MiB | 264.6 MiB | 123.1 MiB (-53.5%; +30.2%) |
| Testify External | `-race` | 32 | 117.4 MiB | 287.2 MiB | 145.9 MiB (-49.2%; +24.3%) |
| Testify External | `-cover` | 4 | 89.9 MiB | 247.8 MiB | 114.6 MiB (-53.8%; +27.4%) |
| Testify External | `-cover` | 32 | 145.0 MiB | 266.8 MiB | 158.1 MiB (-40.7%; +9.0%) |
| Testify External | `-coverpkg=./... -covermode=atomic` | 4 | 88.2 MiB | 242.6 MiB | 113.0 MiB (-53.4%; +28.2%) |
| Testify External | `-coverpkg=./... -covermode=atomic` | 32 | 131.1 MiB | 251.2 MiB | 162.0 MiB (-35.5%; +23.6%) |
| Testify External | `-race -coverpkg=./... -covermode=atomic` | 4 | 94.4 MiB | 268.0 MiB | 122.6 MiB (-54.3%; +29.9%) |
| Testify External | `-race -coverpkg=./... -covermode=atomic` | 32 | 117.9 MiB | 286.8 MiB | 151.6 MiB (-47.1%; +28.6%) |
| Testify External | `-coverpkg=testing,github.com/stretchr/testify/suite -covermode=atomic` | 4 | 87.1 MiB | 246.8 MiB | 117.9 MiB (-52.2%; +35.4%) |
| Testify External | `-coverpkg=testing,github.com/stretchr/testify/suite -covermode=atomic` | 32 | 126.7 MiB | 259.0 MiB | 165.7 MiB (-36.0%; +30.8%) |
| Testify External | `-race -coverpkg=testing,github.com/stretchr/testify/suite -covermode=atomic` | 4 | 92.9 MiB | 263.9 MiB | 121.2 MiB (-54.1%; +30.5%) |
| Testify External | `-race -coverpkg=testing,github.com/stretchr/testify/suite -covermode=atomic` | 32 | 113.8 MiB | 287.0 MiB | 146.6 MiB (-48.9%; +28.8%) |

## Unused-constant edit — diagnostic

| Project | Flags | CPUs | Native | Orchestrion | POC Mini |
| --- | --- | ---: | ---: | ---: | ---: |
| Gin | `none` | 4 | 229.9 MiB | 257.4 MiB | 238.5 MiB (-7.3%; +3.8%) |
| Gin | `none` | 32 | 288.6 MiB | 334.6 MiB | 295.1 MiB (-11.8%; +2.3%) |
| Chi | `none` | 4 | 128.9 MiB | 158.0 MiB | 140.4 MiB (-11.1%; +8.9%) |
| Chi | `none` | 32 | 178.0 MiB | 220.6 MiB | 183.1 MiB (-17.0%; +2.8%) |
| Gin | `-race` | 4 | 232.3 MiB | 253.0 MiB | 241.8 MiB (-4.4%; +4.1%) |
| Gin | `-race` | 32 | 244.6 MiB | 288.1 MiB | 269.8 MiB (-6.4%; +10.3%) |
| Gin | `-cover` | 4 | 245.2 MiB | 384.0 MiB | 257.8 MiB (-32.9%; +5.1%) |
| Gin | `-cover` | 32 | 320.4 MiB | 455.4 MiB | 334.1 MiB (-26.6%; +4.3%) |
| Gin | `-coverpkg=./... -covermode=atomic` | 4 | 259.5 MiB | 401.5 MiB | 266.5 MiB (-33.6%; +2.7%) |
| Gin | `-coverpkg=./... -covermode=atomic` | 32 | 312.0 MiB | 492.1 MiB | 327.9 MiB (-33.4%; +5.1%) |
| Gin | `-race -coverpkg=./... -covermode=atomic` | 4 | 247.5 MiB | 383.5 MiB | 264.7 MiB (-31.0%; +7.0%) |
| Gin | `-race -coverpkg=./... -covermode=atomic` | 32 | 264.1 MiB | 435.3 MiB | 287.4 MiB (-34.0%; +8.8%) |
| Chi | `-race` | 4 | 104.0 MiB | 129.6 MiB | 123.0 MiB (-5.1%; +18.4%) |
| Chi | `-race` | 32 | 126.6 MiB | 155.1 MiB | 136.5 MiB (-12.0%; +7.8%) |
| Chi | `-cover` | 4 | 135.4 MiB | 165.6 MiB | 150.8 MiB (-8.9%; +11.4%) |
| Chi | `-cover` | 32 | 176.3 MiB | 213.5 MiB | 192.9 MiB (-9.7%; +9.4%) |
| Chi | `-coverpkg=./... -covermode=atomic` | 4 | 140.5 MiB | 168.0 MiB | 150.8 MiB (-10.3%; +7.3%) |
| Chi | `-coverpkg=./... -covermode=atomic` | 32 | 172.0 MiB | 208.7 MiB | 193.3 MiB (-7.3%; +12.4%) |
| Chi | `-race -coverpkg=./... -covermode=atomic` | 4 | 110.2 MiB | 141.9 MiB | 120.9 MiB (-14.8%; +9.8%) |
| Chi | `-race -coverpkg=./... -covermode=atomic` | 32 | 134.1 MiB | 174.5 MiB | 141.2 MiB (-19.1%; +5.4%) |
| Testify Direct | `none` | 4 | 80.0 MiB | 101.2 MiB | 88.6 MiB (-12.5%; +10.7%) |
| Testify Direct | `none` | 32 | 133.1 MiB | 146.7 MiB | 142.7 MiB (-2.8%; +7.2%) |
| Testify Direct | `-race` | 4 | 63.1 MiB | 82.3 MiB | 74.3 MiB (-9.7%; +17.9%) |
| Testify Direct | `-race` | 32 | 64.4 MiB | 84.2 MiB | 102.1 MiB (+21.2%; +58.6%) |
| Testify Direct | `-cover` | 4 | 67.5 MiB | 82.8 MiB | 87.8 MiB (+5.9%; +30.0%) |
| Testify Direct | `-cover` | 32 | 93.8 MiB | 101.3 MiB | 142.9 MiB (+41.0%; +52.4%) |
| Testify Direct | `-coverpkg=./... -covermode=atomic` | 4 | 67.5 MiB | 82.2 MiB | 95.6 MiB (+16.3%; +41.6%) |
| Testify Direct | `-coverpkg=./... -covermode=atomic` | 32 | 85.8 MiB | 102.2 MiB | 129.6 MiB (+26.9%; +51.1%) |
| Testify Direct | `-race -coverpkg=./... -covermode=atomic` | 4 | 55.9 MiB | 70.2 MiB | 83.2 MiB (+18.5%; +49.0%) |
| Testify Direct | `-race -coverpkg=./... -covermode=atomic` | 32 | 93.2 MiB | 123.9 MiB | 99.3 MiB (-19.9%; +6.5%) |
| Testify Direct | `-coverpkg=testing,github.com/stretchr/testify/suite -covermode=atomic` | 4 | 77.2 MiB | 110.4 MiB | 89.9 MiB (-18.6%; +16.4%) |
| Testify Direct | `-coverpkg=testing,github.com/stretchr/testify/suite -covermode=atomic` | 32 | 123.9 MiB | 128.1 MiB | 115.2 MiB (-10.0%; -7.0%) |
| Testify Direct | `-race -coverpkg=testing,github.com/stretchr/testify/suite -covermode=atomic` | 4 | 73.7 MiB | 91.5 MiB | 78.5 MiB (-14.2%; +6.5%) |
| Testify Direct | `-race -coverpkg=testing,github.com/stretchr/testify/suite -covermode=atomic` | 32 | 88.8 MiB | 124.8 MiB | 82.0 MiB (-34.3%; -7.6%) |
| Testify External | `none` | 4 | 57.1 MiB | 87.7 MiB | 76.5 MiB (-12.8%; +34.0%) |
| Testify External | `none` | 32 | 87.9 MiB | 144.4 MiB | 79.3 MiB (-45.1%; -9.9%) |
| Testify External | `-race` | 4 | 56.2 MiB | 90.7 MiB | 71.6 MiB (-21.1%; +27.3%) |
| Testify External | `-race` | 32 | 85.1 MiB | 129.7 MiB | 105.9 MiB (-18.3%; +24.5%) |
| Testify External | `-cover` | 4 | 57.0 MiB | 97.6 MiB | 65.3 MiB (-33.1%; +14.6%) |
| Testify External | `-cover` | 32 | 90.2 MiB | 115.7 MiB | 112.6 MiB (-2.7%; +24.8%) |
| Testify External | `-coverpkg=./... -covermode=atomic` | 4 | 57.8 MiB | 86.3 MiB | 69.0 MiB (-20.0%; +19.5%) |
| Testify External | `-coverpkg=./... -covermode=atomic` | 32 | 102.4 MiB | 128.9 MiB | 107.8 MiB (-16.4%; +5.2%) |
| Testify External | `-race -coverpkg=./... -covermode=atomic` | 4 | 53.8 MiB | 84.4 MiB | 65.6 MiB (-22.3%; +22.0%) |
| Testify External | `-race -coverpkg=./... -covermode=atomic` | 32 | 82.3 MiB | 119.7 MiB | 96.0 MiB (-19.8%; +16.6%) |
| Testify External | `-coverpkg=testing,github.com/stretchr/testify/suite -covermode=atomic` | 4 | 54.9 MiB | 94.0 MiB | 67.6 MiB (-28.1%; +23.2%) |
| Testify External | `-coverpkg=testing,github.com/stretchr/testify/suite -covermode=atomic` | 32 | 97.0 MiB | 137.5 MiB | 106.7 MiB (-22.4%; +10.0%) |
| Testify External | `-race -coverpkg=testing,github.com/stretchr/testify/suite -covermode=atomic` | 4 | 56.8 MiB | 93.1 MiB | 62.1 MiB (-33.3%; +9.3%) |
| Testify External | `-race -coverpkg=testing,github.com/stretchr/testify/suite -covermode=atomic` | 32 | 84.5 MiB | 123.5 MiB | 97.2 MiB (-21.3%; +15.0%) |

## Runtime

| Project | Flags | CPUs | Native | Orchestrion | POC Mini | Mini deferred |
| --- | --- | ---: | ---: | ---: | ---: | ---: |
| Gin | `none` | 4 | 36.6 MiB | 60.3 MiB | 52.3 MiB (-13.3%; +42.7%) | 59.6 MiB (-1.2%; +62.6%) |
| Gin | `none` | 32 | 42.2 MiB | 70.7 MiB | 80.4 MiB (+13.7%; +90.5%) | 74.2 MiB (+4.9%; +75.9%) |
| Chi | `none` | 4 | 45.6 MiB | 79.7 MiB | 60.5 MiB (-24.1%; +32.6%) | 60.8 MiB (-23.7%; +33.2%) |
| Chi | `none` | 32 | 94.7 MiB | 128.0 MiB | 96.0 MiB (-25.0%; +1.3%) | 85.5 MiB (-33.3%; -9.8%) |
| Gin | `-race` | 4 | 199.3 MiB | FAIL 5/6 | 206.8 MiB (vs Orchestrion unavailable; +3.8% vs Native) | 210.9 MiB (vs Orchestrion unavailable; +5.8% vs Native) |
| Gin | `-race` | 32 | 262.9 MiB | FAIL 6/6 | 392.7 MiB (vs Orchestrion unavailable; +49.4% vs Native) | 393.9 MiB (vs Orchestrion unavailable; +49.8% vs Native) |
| Gin | `-cover` | 4 | 35.9 MiB | 92.6 MiB | 60.2 MiB (-35.0%; +67.9%) | 54.8 MiB (-40.8%; +52.8%) |
| Gin | `-cover` | 32 | 42.1 MiB | 165.2 MiB | 79.3 MiB (-52.0%; +88.4%) | 83.8 MiB (-49.3%; +98.9%) |
| Gin | `-coverpkg=./... -covermode=atomic` | 4 | 35.7 MiB | 98.6 MiB | 117.1 MiB (+18.8%; +227.8%) | 122.6 MiB (+24.3%; +243.2%) |
| Gin | `-coverpkg=./... -covermode=atomic` | 32 | 52.1 MiB | 200.9 MiB | 218.4 MiB (+8.7%; +319.3%) | 224.4 MiB (+11.7%; +330.8%) |
| Gin | `-race -coverpkg=./... -covermode=atomic` | 4 | 243.9 MiB | FAIL 5/6 | UNVERIFIED | UNVERIFIED |
| Gin | `-race -coverpkg=./... -covermode=atomic` | 32 | 283.8 MiB | FAIL 5/6 | UNVERIFIED | UNVERIFIED |
| Chi | `-race` | 4 | 2958.8 MiB | 2979.4 MiB | 3023.4 MiB (+1.5%; +2.2%) | 3019.0 MiB (+1.3%; +2.0%) |
| Chi | `-race` | 32 | 3068.8 MiB | 3112.4 MiB | 3098.3 MiB (-0.5%; +1.0%) | 3111.2 MiB (-0.0%; +1.4%) |
| Chi | `-cover` | 4 | 43.0 MiB | 85.0 MiB | 52.6 MiB (-38.1%; +22.2%) | 60.1 MiB (-29.2%; +39.8%) |
| Chi | `-cover` | 32 | 88.6 MiB | 149.1 MiB | 93.2 MiB (-37.5%; +5.2%) | 87.2 MiB (-41.5%; -1.6%) |
| Chi | `-coverpkg=./... -covermode=atomic` | 4 | 45.1 MiB | 98.8 MiB | 59.8 MiB (-39.5%; +32.5%) | 63.6 MiB (-35.6%; +40.9%) |
| Chi | `-coverpkg=./... -covermode=atomic` | 32 | 84.1 MiB | 145.2 MiB | 100.5 MiB (-30.8%; +19.4%) | 103.8 MiB (-28.5%; +23.4%) |
| Chi | `-race -coverpkg=./... -covermode=atomic` | 4 | 2944.4 MiB | 3073.4 MiB | 2481.0 MiB (-19.3%; -15.7%) | 2478.9 MiB (-19.3%; -15.8%) |
| Chi | `-race -coverpkg=./... -covermode=atomic` | 32 | 2940.6 MiB | 3072.5 MiB | 2503.1 MiB (-18.5%; -14.9%) | 2494.0 MiB (-18.8%; -15.2%) |
| Testify Direct | `none` | 4 | 14.5 MiB | 26.6 MiB | 17.3 MiB (-35.2%; +18.6%) | 15.4 MiB (-42.0%; +6.2%) |
| Testify Direct | `none` | 32 | 15.0 MiB | 29.4 MiB | 19.2 MiB (-34.6%; +28.5%) | 18.4 MiB (-37.5%; +22.7%) |
| Testify Direct | `-race` | 4 | 34.1 MiB | 67.5 MiB | 42.7 MiB (-36.7%; +25.2%) | 42.4 MiB (-37.1%; +24.5%) |
| Testify Direct | `-race` | 32 | 37.2 MiB | 73.4 MiB | 44.9 MiB (-38.8%; +20.7%) | 44.1 MiB (-39.9%; +18.6%) |
| Testify Direct | `-cover` | 4 | 14.5 MiB | 29.7 MiB | 17.5 MiB (-41.1%; +20.8%) | 17.4 MiB (-41.4%; +20.2%) |
| Testify Direct | `-cover` | 32 | 14.7 MiB | 33.8 MiB | 20.4 MiB (-39.6%; +38.7%) | 17.9 MiB (-47.0%; +21.5%) |
| Testify Direct | `-coverpkg=./... -covermode=atomic` | 4 | 14.3 MiB | 27.6 MiB | 22.2 MiB (-19.5%; +55.2%) | 22.2 MiB (-19.8%; +54.7%) |
| Testify Direct | `-coverpkg=./... -covermode=atomic` | 32 | 14.8 MiB | 34.0 MiB | 25.3 MiB (-25.6%; +71.5%) | 25.6 MiB (-24.9%; +73.1%) |
| Testify Direct | `-race -coverpkg=./... -covermode=atomic` | 4 | 34.0 MiB | 66.7 MiB | 45.4 MiB (-31.9%; +33.6%) | 44.6 MiB (-33.1%; +31.4%) |
| Testify Direct | `-race -coverpkg=./... -covermode=atomic` | 32 | 35.8 MiB | 72.1 MiB | 51.6 MiB (-28.4%; +44.4%) | 50.3 MiB (-30.3%; +40.6%) |
| Testify Direct | `-coverpkg=testing,github.com/stretchr/testify/suite -covermode=atomic` | 4 | 14.8 MiB | 27.7 MiB | 21.0 MiB (-24.3%; +41.8%) | 22.0 MiB (-20.8%; +48.3%) |
| Testify Direct | `-coverpkg=testing,github.com/stretchr/testify/suite -covermode=atomic` | 32 | 15.0 MiB | 35.2 MiB | 25.3 MiB (-28.1%; +69.0%) | 25.0 MiB (-29.0%; +66.9%) |
| Testify Direct | `-race -coverpkg=testing,github.com/stretchr/testify/suite -covermode=atomic` | 4 | 35.3 MiB | 74.4 MiB | 49.6 MiB (-33.4%; +40.3%) | 49.8 MiB (-33.1%; +41.0%) |
| Testify Direct | `-race -coverpkg=testing,github.com/stretchr/testify/suite -covermode=atomic` | 32 | 36.7 MiB | 83.5 MiB | 56.7 MiB (-32.2%; +54.3%) | 56.1 MiB (-32.8%; +52.8%) |
| Testify External | `none` | 4 | 14.3 MiB | 27.1 MiB | 17.0 MiB (-37.3%; +18.6%) | 17.1 MiB (-37.0%; +19.1%) |
| Testify External | `none` | 32 | 14.5 MiB | 32.3 MiB | 18.9 MiB (-41.5%; +30.5%) | 18.1 MiB (-44.0%; +24.9%) |
| Testify External | `-race` | 4 | 33.8 MiB | 68.8 MiB | 42.9 MiB (-37.7%; +26.9%) | 41.2 MiB (-40.1%; +21.8%) |
| Testify External | `-race` | 32 | 37.0 MiB | 75.3 MiB | 45.2 MiB (-39.9%; +22.3%) | 44.5 MiB (-40.9%; +20.5%) |
| Testify External | `-cover` | 4 | 14.2 MiB | 29.4 MiB | 17.3 MiB (-41.3%; +21.3%) | 17.5 MiB (-40.6%; +22.7%) |
| Testify External | `-cover` | 32 | 15.0 MiB | 34.4 MiB | 18.8 MiB (-45.3%; +25.3%) | 17.8 MiB (-48.3%; +18.4%) |
| Testify External | `-coverpkg=./... -covermode=atomic` | 4 | 14.3 MiB | 28.2 MiB | 23.9 MiB (-15.1%; +66.9%) | 21.8 MiB (-22.7%; +51.9%) |
| Testify External | `-coverpkg=./... -covermode=atomic` | 32 | 15.0 MiB | 34.4 MiB | 25.8 MiB (-25.1%; +71.6%) | 25.0 MiB (-27.2%; +66.7%) |
| Testify External | `-race -coverpkg=./... -covermode=atomic` | 4 | 32.8 MiB | 67.1 MiB | 45.2 MiB (-32.7%; +37.7%) | 44.6 MiB (-33.6%; +35.9%) |
| Testify External | `-race -coverpkg=./... -covermode=atomic` | 32 | 35.5 MiB | 73.1 MiB | 51.6 MiB (-29.4%; +45.3%) | 50.8 MiB (-30.4%; +43.2%) |
| Testify External | `-coverpkg=testing,github.com/stretchr/testify/suite -covermode=atomic` | 4 | 14.6 MiB | 28.0 MiB | 24.2 MiB (-13.7%; +65.6%) | 22.5 MiB (-19.5%; +54.5%) |
| Testify External | `-coverpkg=testing,github.com/stretchr/testify/suite -covermode=atomic` | 32 | 16.8 MiB | 34.5 MiB | 25.6 MiB (-25.8%; +52.8%) | 26.5 MiB (-23.2%; +58.1%) |
| Testify External | `-race -coverpkg=testing,github.com/stretchr/testify/suite -covermode=atomic` | 4 | 35.3 MiB | 74.3 MiB | 49.4 MiB (-33.5%; +40.1%) | 49.7 MiB (-33.1%; +40.9%) |
| Testify External | `-race -coverpkg=testing,github.com/stretchr/testify/suite -covermode=atomic` | 32 | 36.3 MiB | 73.0 MiB | 58.8 MiB (-19.5%; +62.0%) | 54.4 MiB (-25.5%; +49.9%) |

## Gin: local Agent delivery and telemetry

Three measured runs follow one warmup.

| Project | Flags | CPUs | Native | Orchestrion | POC Mini |
| --- | --- | ---: | ---: | ---: | ---: |
| Gin | `none` | 4 | 41.4 MiB | 77.5 MiB | 64.8 MiB (-16.4%; +56.4%) |
| Gin | `none` | 32 | 41.5 MiB | 78.2 MiB | 72.9 MiB (-6.8%; +75.5%) |
