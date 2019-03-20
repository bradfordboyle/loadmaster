# loadmaster

> A loadmaster is an aircrew member on civilian aircraft or military transport aircraft tasked with the safe loading, transport and unloading of aerial cargoes.
>
> &mdash; <cite>Wikipedia</cite>

This is a _simple_ tool for reading a Concourse pipeline configuration and downloading resources from blob store.

```sh
fly -t <target> get-pipeline --pipeline <pipeline-name> > pipeline.yml
./loadmaster pipeline.yml
```

## Limitations

This is still a _proof of concept_; here are the limitations:

1. Only works w/ the [`gcs` resource](https://github.com/frodenas/gcs-resource)
2. Downloads all the supported resources
3. Requires credentials to already be interpolated in the pipeline config
