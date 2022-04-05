local images = import 'images.libsonnet';
{
  step(name, commands, image=images._images.go, settings={}):: {
    name: name,
    commands: commands,
    image: image,
    settings: settings,
  },

  withStep(step):: {
    steps+: [step],
  },

  withInlineStep(name, commands, image=images._images.go, settings={}, environment={}):: $.withStep($.step(name, commands, image, settings) + environment),

  pipeline(name, steps=[], depends_on=null):: {
    kind: 'pipeline',
    type: 'docker',
    name: name,
    steps: steps,
    depends_on: depends_on,
  },
}
