# Notes during workshop

LSM Tree : log structured merge tree

- usually append only
- write 

## Components

1. memtable
2. immutable memtable - frozen memtable
3. sorted strings table on disk
4. WAL 
5. statefile [file with metadata]


## Write Flow

1. write first to memtable as well as WAL
2. freeze memtable to become immutable when the memtable is too large
3. immutable memtable written to disk as SStable 


## Read Flow

1. memtable 
2. immutable memtable
3. sstable files


## Optimization Params

1. read amplifications
2. write amplifications
3. space amplifications


## Compaction

- Merge SStables into one
- Merging all the versions of keys into 1
- Dropping some deleted keys


## Stage

- [ ] memtable
- [ ] reading from and writing to disk, flush
- [ ] reading across levels
- [ ] compaction in isolation
- [ ] compaction integrated 