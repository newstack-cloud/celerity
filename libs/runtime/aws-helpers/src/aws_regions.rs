use aws_config::meta::region::future;
use aws_config::meta::region::ProvideRegion;
use aws_types::region::Region;

#[derive(Debug)]
pub struct RegionProvider {
    aws_region: String,
}

impl RegionProvider {
    pub fn new(aws_region: String) -> RegionProvider {
        RegionProvider { aws_region }
    }
}

impl ProvideRegion for RegionProvider {
    fn region(&self) -> future::ProvideRegion<'_> {
        future::ProvideRegion::ready(Some(Region::new(self.aws_region.clone())))
    }
}
