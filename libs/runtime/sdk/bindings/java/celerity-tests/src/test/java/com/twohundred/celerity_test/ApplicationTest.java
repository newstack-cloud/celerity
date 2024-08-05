package com.twohundred.celerity_test;

import com.twohundred.celerity.Application;
import org.junit.jupiter.api.Test;

import java.util.concurrent.ExecutionException;

import static org.assertj.core.api.Assertions.assertThat;
import static org.joou.Unsigned.uint;

public class ApplicationTest {
    @Test
    public void ConstructionDestructionTest() {
        assertThat(Application.constructionCounter().intValue()).isZero();

        Application application = new Application(uint(41));
        assertThat(Application.constructionCounter()).isEqualTo(uint(1));
        assertThat(application.getValue()).isEqualTo(uint(200));

        application.shutdown();

        assertThat(Application.constructionCounter().intValue()).isZero();
    }
}
