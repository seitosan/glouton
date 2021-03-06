import { gql } from 'apollo-boost'
import { LabelName } from '.'

// FACTS AND DETAILS

export const FACTS = gql`
  query facts {
    facts {
      name
      value
    }
  }
`

export const AGENT_DETAILS = gql`
  query agent_details {
    services(isActive: true) {
      name
      containerId
      ipAddress
      listenAddresses
      exePath
      active
      status
      statusDescription
    }
    agentInformation {
      registrationAt
      lastReport
      isConnected
    }
    tags {
      tagName
    }
    agentStatus {
      status
      statusDescription
    }
  }
`

// CONTAINERS

export const CONTAINERS_DETAILS = gql`
  query containersDetails($offset: Int!, $limit: Int!, $allContainers: Boolean!, $search: String!) {
    containers(input: { offset: $offset, limit: $limit }, allContainers: $allContainers, search: $search) {
      count
      currentCount
      containers {
        command
        createdAt
        id
        image
        inspectJSON
        name
        startedAt
        state
        finishedAt
        ioWriteBytes
        ioReadBytes
        netBitsRecv
        netBitsSent
        memUsedPerc
        cpuUsedPerc
      }
    }
  }
`

export const CONTAINER_PROCESSES = gql`
  query containerProcesses($containerId: String!) {
    processes(containerId: $containerId) {
      processes {
        pid
        cmdline
        name
        memory_rss
        cpu_percent
        cpu_time
        status
        username
      }
    }
    points(
      metricsFilter: [
        { labels: { key: "${LabelName}", value: "mem_buffered" } }
        { labels: { key: "${LabelName}", value: "mem_cached" } }
        { labels: { key: "${LabelName}", value: "mem_free" } }
        { labels: { key: "${LabelName}", value: "mem_used" } }
      ]
      start: ""
      end: ""
      minutes: 15
    ) {
      labels {
        key
        value
      }
      points {
        time
        value
      }
    }
  }
`

export const CONTAINER_SERVICE = gql`
  query containerService($containerId: String!) {
    containers(search: $containerId, allContainers: true) {
      containers {
        name
      }
    }
  }
`

// PROCESSES

export const PROCESSES = gql`
  query processesQuery {
    processes {
      updatedAt
      processes {
        pid
        ppid
        cmdline
        name
        memory_rss
        cpu_percent
        cpu_time
        status
        username
      }
    }
    points(
      metricsFilter: [
        { labels: { key: "${LabelName}", value: "mem_buffered" } }
        { labels: { key: "${LabelName}", value: "mem_cached" } }
        { labels: { key: "${LabelName}", value: "mem_free" } }
        { labels: { key: "${LabelName}", value: "mem_used" } }
        { labels: { key: "${LabelName}", value: "system_load1" } }
        { labels: { key: "${LabelName}", value: "system_load5" } }
        { labels: { key: "${LabelName}", value: "system_load15" } }
        { labels: { key: "${LabelName}", value: "swap_free" } }
        { labels: { key: "${LabelName}", value: "swap_used" } }
        { labels: { key: "${LabelName}", value: "swap_total" } }
        { labels: { key: "${LabelName}", value: "cpu_system" } }
        { labels: { key: "${LabelName}", value: "cpu_user" } }
        { labels: { key: "${LabelName}", value: "cpu_nice" } }
        { labels: { key: "${LabelName}", value: "cpu_wait" } }
        { labels: { key: "${LabelName}", value: "cpu_idle" } }
        { labels: { key: "${LabelName}", value: "uptime" } }
        { labels: { key: "${LabelName}", value: "users_logged" } }
      ]
      start: ""
      end: ""
      minutes: 15
    ) {
      labels {
        key
        value
      }
      points {
        time
        value
      }
    }
  }
`

// GRAPHS

export const GET_POINTS = gql`
  query Points($metricsFilter: [MetricInput!]!, $start: String!, $end: String!, $minutes: Int!) {
    points(metricsFilter: $metricsFilter, start: $start, end: $end, minutes: $minutes) {
      labels {
        key
        value
      }
      points {
        time
        value
      }
      thresholds {
        highWarning
        highCritical
      }
    }
  }
`
