{
  main:: {
    trigger+: {
      branch+: ['main'],
      event+: {
        include+: ['push'],
      },
    },
  },
  pr:: {
    trigger+: {
      event+: {
        include+: ['pull_request'],
      },
    },
  },
}
