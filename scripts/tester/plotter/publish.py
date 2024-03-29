#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
Created on Thu Mar 30 09:58:32 2023

@author: yasi
"""

from matplotlib import pyplot as plt
import numpy as np
import csv

def read(fileName, topic):
    init_sizes = []
    batch_sizes = []
    pings = []
    latency = []
    
    with open(fileName) as file:        
        reader = csv.reader(file)
        line_index = 0
        for row in reader:
            if line_index > 0:
                if row[0] != topic:
                    continue
                if float(row[4]) != 100.0:
                    continue
                init_sizes.append(int(row[1]))
                batch_sizes.append(int(row[2]))
                pings.append(int(row[3]))
                latency.append(float(row[5]))
            line_index += 1
    return init_sizes, batch_sizes, pings, latency

def findDisctnctBatches(batch_sizes):
    dist_batchs = []
    tmp_batch = -1
    for i in range(len(batch_sizes)):
        if tmp_batch != batch_sizes[i]:
            tmp_batch = batch_sizes[i]
            exists = 0
            for j in range(len(dist_batchs)):
                if dist_batchs[j] == tmp_batch:
                    exists = 1
                    break
            if exists == 0:
                dist_batchs.append(tmp_batch)
    return dist_batchs

def parse(filtr_batch, init_sizes, batch_sizes, pings, latency):
    sizes = []
    avg_lat_list = []
    err_list = []
    
    tmp_size = init_sizes[0]
    tmp_lat_list = []
    for i in range(len(latency)):
        if batch_sizes[i] != filtr_batch:
            continue
        if tmp_size != init_sizes[i]:
            avg_lat_list.append(np.mean(tmp_lat_list))
            err_list.append(np.std(tmp_lat_list))
            sizes.append(tmp_size)
            tmp_size = init_sizes[i]
            tmp_lat_list = [latency[i]-pings[i]]
        else:
            tmp_lat_list.append(latency[i]-pings[i])

    avg_lat_list.append(np.mean(tmp_lat_list))
    err_list.append(np.std(tmp_lat_list))
    sizes.append(tmp_size)
    
    return sizes, avg_lat_list, err_list

def plot(sizes_s, sizes_m, avg_lat_list_s, avg_lat_list_m, err_list_s, err_list_m, clrs, labels):    
    fig, ax = plt.subplots()
    j = 0
    
    for i in range(len(sizes_s)):
        ax.errorbar(sizes_s[i], avg_lat_list_s[i], yerr=err_list_s[i], ecolor="red", color=clrs[j], label=labels[j], ls='-.')
        j += 1
        
    for i in range(len(sizes_m)):
        ax.errorbar(sizes_m[i], avg_lat_list_m[i], yerr=err_list_m[i], ecolor="red", color=clrs[j], label=labels[j])
        j += 1
    
    ax.set_xlabel('initial group size')
    ax.set_ylabel('average time taken (ms)')
    ax.set_title('Latency and throughput results for group-message')
    plt.rc('grid', linestyle="--", color='#C6C6C6')
    plt.legend(bbox_to_anchor=(1.1, 1.05))
    plt.grid()
    plt.savefig('../../../docs/publish_latency.pdf', bbox_inches="tight")
    plt.show()


# for latency graph
init_sizes_s, batch_sizes_s, pings_s, latency_s = read('../results/publish_latency.csv', 'sq-c-o-topic')
init_sizes_m, batch_sizes_m, pings_m, latency_m = read('../results/publish_latency.csv', 'mq-c-o-topic')

dist_batchs_s = findDisctnctBatches(batch_sizes_s)
sizes_s = []
avg_lat_list_s = []
err_list_s = []
index = 0

for i in range(len(dist_batchs_s)):
    tmp_sizes, tmp_avg_list, tmp_err_list = parse(dist_batchs_s[i], init_sizes_s, batch_sizes_s, pings_s, latency_s)
    if len(tmp_sizes) == 0:
        continue
    sizes_s.append(tmp_sizes)
    avg_lat_list_s.append(tmp_avg_list)
    err_list_s.append(tmp_err_list)
    index += 1
    
dist_batchs_m = findDisctnctBatches(batch_sizes_m)
sizes_m = []
avg_lat_list_m = []
err_list_m = []
index = 0

# to eliminate last 3 items of multiple-queue
init_sizes_m = init_sizes_m[:-1]
latency_m = latency_m[:-1]

for i in range(len(dist_batchs_m)):
    tmp_sizes, tmp_avg_list, tmp_err_list = parse(dist_batchs_m[i], init_sizes_m, batch_sizes_m, pings_m, latency_m)
    if len(tmp_sizes) == 0:
        continue
    sizes_m.append(tmp_sizes)
    avg_lat_list_m.append(tmp_avg_list)
    err_list_m.append(tmp_err_list)
    index += 1
 
clrs = ["navy", "darkgreen", "darkgoldenrod", "purple", "cornflowerblue", "darkseagreen", "khaki", "plum"]
labels = ["batch=1,single-queue", "batch=10,single-queue", "batch=50,single-queue", "batch=100,single-queue", "batch=1,multiple-queue", "batch=10,multiple-queue", "batch=50,multiple-queue", "batch=100,multiple-queue"]
plot(sizes_s, sizes_m, avg_lat_list_s, avg_lat_list_m, err_list_s, err_list_m, clrs, labels)

# for success-rate graph
def readSuccess(fileName, topic):
    init_sizes = []
    batch_sizes = []
    sucs_list = []
    
    with open(fileName) as file:        
        reader = csv.reader(file)
        line_index = 0
        for row in reader:
            if line_index > 0:
                if row[0] != topic:
                    continue
                init_sizes.append(int(row[1]))
                batch_sizes.append(int(row[2]))
                sucs_list.append(float(row[4]))
            line_index += 1
    return init_sizes, batch_sizes, sucs_list

sucs_init_sizes_s, sucs_batch_sizes_s, sucs_list_s = readSuccess('../results/publish_latency.csv', 'sq-c-o-topic')
sucs_init_sizes_m, sucs_batch_sizes_m, sucs_list_m = readSuccess('../results/publish_latency.csv', 'mq-c-o-topic')

def parseSuccess(filtr_batch, init_sizes, batch_sizes, sucs_list):
    sizes = []
    avg_sucs_list = []
    err_list = []
    
    tmp_size = init_sizes[0]
    tmp_sucs_list = []
    for i in range(len(sucs_list)):
        if batch_sizes[i] != filtr_batch:
            continue
        if tmp_size != init_sizes[i]:
            avg_sucs_list.append(np.mean(tmp_sucs_list))
            err_list.append(np.std(tmp_sucs_list))
            sizes.append(tmp_size)
            tmp_size = init_sizes[i]
            tmp_sucs_list = [sucs_list[i]]
        else:
            tmp_sucs_list.append(sucs_list[i])

    avg_sucs_list.append(np.mean(tmp_sucs_list))
    err_list.append(np.std(tmp_sucs_list))
    sizes.append(tmp_size)
    return sizes, avg_sucs_list, err_list    

def plotSuccess(sizes_s, sizes_m, avg_sucs_list_s, avg_sucs_list_m, sucs_err_list_s, sucs_err_list_m, clrs, labels):
    fig, ax = plt.subplots()
    j = 0
    for i in range(len(sizes_s)):
        ax.errorbar(sizes_s[i], avg_sucs_list_s[i], yerr=sucs_err_list_s[i], ecolor="red", color=clrs[j], label=labels[j], ls='-.')
        j += 1
        
    for i in range(len(sizes_m)):
        ax.errorbar(sizes_m[i], avg_sucs_list_m[i], yerr=sucs_err_list_m[i], ecolor="red", color=clrs[j], label=labels[j])
        j += 1
    
    ax.set_xlabel('initial group size')
    ax.set_ylabel('success rate (%)')
    ax.set_title('Success rates of group-message')
    ax.set_ylim(ymin=0)
    plt.rc('grid', linestyle="--", color='#C6C6C6')
    plt.legend(bbox_to_anchor=(1.1, 1.05))
    plt.grid()
    plt.savefig('../../../docs/publish_success.pdf', bbox_inches="tight")
    plt.show()
    
sucs_init_sizes_s, sucs_batch_sizes_s, sucs_list_s = readSuccess('../results/publish_latency.csv', 'sq-c-o-topic')
sucs_init_sizes_m, sucs_batch_sizes_m, sucs_list_m = readSuccess('../results/publish_latency.csv', 'mq-c-o-topic')

sucs_dist_batchs_s = findDisctnctBatches(sucs_batch_sizes_s)
sucs_sizes_s = []
sucs_avg_lat_list_s = []
sucs_err_list_s = []
index = 0

for i in range(len(sucs_dist_batchs_s)):
    tmp_sizes, tmp_avg_list, tmp_err_list = parseSuccess(sucs_dist_batchs_s[i], sucs_init_sizes_s, sucs_batch_sizes_s, sucs_list_s)
    if len(tmp_sizes) == 0:
        continue
    sucs_sizes_s.append(tmp_sizes)
    sucs_avg_lat_list_s.append(tmp_avg_list)
    sucs_err_list_s.append(tmp_err_list)
    index += 1

sucs_dist_batchs_m = findDisctnctBatches(sucs_batch_sizes_m)
sucs_sizes_m = []
sucs_avg_lat_list_m = []
sucs_err_list_m = []
index = 0

for i in range(len(sucs_dist_batchs_m)):
    tmp_sizes, tmp_avg_list, tmp_err_list = parseSuccess(sucs_dist_batchs_m[i], sucs_init_sizes_m, sucs_batch_sizes_m, sucs_list_m)
    if len(tmp_sizes) == 0:
        continue
    sucs_sizes_m.append(tmp_sizes)
    sucs_avg_lat_list_m.append(tmp_avg_list)
    sucs_err_list_m.append(tmp_err_list)
    index += 1

plotSuccess(sucs_sizes_s, sucs_sizes_m, sucs_avg_lat_list_s, sucs_avg_lat_list_m, sucs_err_list_s, sucs_err_list_m, clrs, labels)

# for single node graph

def plotSingle(batch_list, avg_lat_list, err_list, name):    
    fig, ax = plt.subplots()    
    ax.errorbar(batch_list, avg_lat_list, yerr=err_list, ecolor="red")
    
    ax.set_xlabel('number of messages')
    ax.set_ylabel('average time taken (ms)')
    ax.set_title('Latency for group messages with a single node')
    plt.grid()
    plt.savefig('../../../docs/' + name + '.pdf', bbox_inches="tight")
    plt.show()

init_sizes_s, batch_sizes_s, pings_s, latency_s = read('../results/publish_latency_single.csv', 'sq-c-o-topic')

dist_batchs_s = findDisctnctBatches(batch_sizes_s)
avg_lat_list_s = []
err_list_s = []
index = 0

for i in range(len(dist_batchs_s)):
    tmp_sizes, tmp_avg_list, tmp_err_list = parse(dist_batchs_s[i], init_sizes_s, batch_sizes_s, pings_s, latency_s)
    if len(tmp_sizes) == 0:
        continue
    avg_lat_list_s.append(tmp_avg_list[0])
    err_list_s.append(tmp_err_list[0])
    index += 1

# for small batches
small_batchs_s = []
small_avg_lat_list_s = []
small_err_list_s = []    
for i in range(len(dist_batchs_s)):
    if dist_batchs_s[i] > 100:
        continue
    small_batchs_s.append(dist_batchs_s[i])
    small_avg_lat_list_s.append(avg_lat_list_s[i])
    small_err_list_s.append(err_list_s [i])

plotSingle(small_batchs_s, small_avg_lat_list_s, small_err_list_s, 'publish_latency_single_small')

# for large batches
large_batchs_s = []
large_avg_lat_list_s = []
large_err_list_s = []    
for i in range(len(dist_batchs_s)):
    if dist_batchs_s[i] < 100:
        continue
    large_batchs_s.append(dist_batchs_s[i])
    large_avg_lat_list_s.append(avg_lat_list_s[i])
    large_err_list_s.append(err_list_s [i])

plotSingle(large_batchs_s, large_avg_lat_list_s, large_err_list_s, 'publish_latency_single_large')







